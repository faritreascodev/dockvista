package service

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

// maxCleanupItems caps one plan. It is far above what the UI sends and only
// exists so a single request cannot queue an unbounded number of daemon
// calls.
const maxCleanupItems = 1000

// cleanupTimeout bounds a whole cleanup run. The run is detached from the
// request so a closed tab does not leave half a plan executed.
const cleanupTimeout = 10 * time.Minute

// StorageService itemizes disk usage and removes what the caller selected,
// after re-checking every item against a fresh inventory.
//
// The rules: a running, paused, or restarting container is never removed.
// An image or volume is removable only when nothing references it, or when
// every container that references it is removed in the same plan. Anything
// labelled dockvista.protect=true is never removed. Removal never forces
// past the daemon's own in-use checks.
type StorageService struct {
	docker ports.StorageClient
	// onChange runs after a cleanup removed something, so cached disk
	// usage is not served stale.
	onChange func()
	now      func() time.Time

	// runMu serializes cleanups: two overlapping plans would validate
	// against inventories that the other is changing.
	runMu sync.Mutex
}

func NewStorageService(docker ports.StorageClient, onChange func()) *StorageService {
	if onChange == nil {
		onChange = func() {}
	}
	return &StorageService{docker: docker, onChange: onChange, now: time.Now}
}

func (s *StorageService) Inventory(ctx context.Context) (domain.StorageInventory, error) {
	inv, err := s.docker.StorageInventory(ctx)
	if err != nil {
		return domain.StorageInventory{}, fmt.Errorf("service: storage inventory: %w", err)
	}
	inv.GeneratedAt = s.now().UTC()
	classify(inv.Items, nil)
	slices.SortStableFunc(inv.Items, func(a, b domain.StorageItem) int {
		if a.Kind != b.Kind {
			return cmp.Compare(kindOrder(a.Kind), kindOrder(b.Kind))
		}
		return cmp.Compare(b.SizeBytes, a.SizeBytes)
	})
	return inv, nil
}

// Cleanup validates plan against a fresh inventory and, unless dryRun,
// removes the accepted items. Items that fail validation are reported as
// skipped, never silently dropped.
func (s *StorageService) Cleanup(ctx context.Context, plan domain.CleanupPlan, dryRun bool) (domain.CleanupReport, error) {
	if plan.Size() == 0 || plan.Size() > maxCleanupItems {
		return domain.CleanupReport{}, fmt.Errorf("service: cleanup plan: %w", domain.ErrInvalidInput)
	}
	if !dryRun {
		s.runMu.Lock()
		defer s.runMu.Unlock()
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
	}

	inv, err := s.docker.StorageInventory(ctx)
	if err != nil {
		return domain.CleanupReport{}, fmt.Errorf("service: storage inventory: %w", err)
	}

	steps, report := buildCleanup(inv.Items, plan)
	report.DryRun = dryRun
	if dryRun {
		for _, st := range steps {
			report.Results = append(report.Results, st.result(domain.CleanupPlanned, "", st.item.SizeBytes))
			report.FreedBytes += st.item.SizeBytes
		}
		return report, nil
	}

	removed := map[string]bool{}
	anyRemoved := false
	for _, st := range steps {
		if blocker := firstMissing(st.item.UsedBy, removed); st.item.Kind != domain.StorageContainer && blocker != "" {
			report.Results = append(report.Results, st.result(domain.CleanupSkipped,
				"kept: a container that uses it ("+shortID(blocker)+") was not removed", 0))
			continue
		}
		freed, err := s.remove(ctx, st.item)
		if err != nil {
			report.Results = append(report.Results, st.result(domain.CleanupFailed, err.Error(), 0))
			continue
		}
		removed[st.item.ID] = true
		anyRemoved = true
		report.Results = append(report.Results, st.result(domain.CleanupRemoved, "", freed))
		report.FreedBytes += freed
	}
	if anyRemoved {
		s.onChange()
	}
	return report, nil
}

func (s *StorageService) remove(ctx context.Context, item domain.StorageItem) (int64, error) {
	switch item.Kind {
	case domain.StorageContainer:
		return item.SizeBytes, s.docker.RemoveContainer(ctx, item.ID, false)
	case domain.StorageImage:
		// Removing an image by ID with several tags needs force; that is
		// only safe because validation already proved no container,
		// stopped or not, still uses it.
		return item.SizeBytes, s.docker.RemoveImage(ctx, item.ID, len(item.Tags) > 1)
	case domain.StorageVolume:
		return item.SizeBytes, s.docker.RemoveVolume(ctx, item.ID, false)
	case domain.StorageBuildCache:
		freed, err := s.docker.PruneBuildCache(ctx)
		return int64(min(freed, uint64(1<<62))), err
	}
	return 0, fmt.Errorf("unknown kind %q: %w", item.Kind, domain.ErrInvalidInput)
}

type cleanupStep struct{ item domain.StorageItem }

func (st cleanupStep) result(status domain.CleanupStatus, msg string, freed int64) domain.CleanupResult {
	return domain.CleanupResult{
		Kind: st.item.Kind, ID: st.item.ID, Name: st.item.Name,
		Status: status, Message: msg, FreedBytes: freed,
	}
}

// buildCleanup turns a plan into ordered steps (containers, then images,
// then volumes, then build cache, so references disappear before the
// objects they point at) plus skipped results for everything rejected.
func buildCleanup(items []domain.StorageItem, plan domain.CleanupPlan) ([]cleanupStep, domain.CleanupReport) {
	var report domain.CleanupReport
	index := make(map[string]int, len(items))
	for i, it := range items {
		index[storageKey(it.Kind, it.ID)] = i
	}
	// lookup reads through to items so it sees the latest classify pass.
	lookup := func(kind domain.StorageKind, id string) (domain.StorageItem, bool) {
		i, ok := index[storageKey(kind, id)]
		if !ok {
			return domain.StorageItem{}, false
		}
		return items[i], true
	}

	skip := func(kind domain.StorageKind, id, name, why string) {
		if name == "" {
			name = id
		}
		report.Results = append(report.Results, domain.CleanupResult{
			Kind: kind, ID: id, Name: name, Status: domain.CleanupSkipped, Message: why,
		})
	}

	// Containers are judged on their own, then the accepted set lets
	// images and volumes count those containers as already gone.
	classify(items, nil)
	var steps []cleanupStep
	removing := map[string]bool{}
	for _, id := range dedupe(plan.Containers) {
		it, ok := lookup(domain.StorageContainer, id)
		switch {
		case !ok:
			skip(domain.StorageContainer, id, "", "no longer exists")
		case !it.Eligible:
			skip(domain.StorageContainer, id, it.Name, it.Reason)
		default:
			removing[id] = true
			steps = append(steps, cleanupStep{it})
		}
	}

	classify(items, removing)
	for _, group := range []struct {
		kind domain.StorageKind
		ids  []string
	}{{domain.StorageImage, plan.Images}, {domain.StorageVolume, plan.Volumes}} {
		for _, id := range dedupe(group.ids) {
			it, ok := lookup(group.kind, id)
			if !ok {
				skip(group.kind, id, "", "no longer exists")
				continue
			}
			if !it.Eligible {
				skip(group.kind, id, it.Name, it.Reason)
				continue
			}
			steps = append(steps, cleanupStep{it})
		}
	}

	if plan.BuildCache {
		var reclaim int64
		for _, it := range items {
			// Shared records back other records; `docker system df` leaves
			// them out of reclaimable, and so does this estimate.
			if it.Kind == domain.StorageBuildCache && it.Eligible && !it.Shared {
				reclaim += it.SizeBytes
			}
		}
		if reclaim == 0 {
			skip(domain.StorageBuildCache, "unused", "Unused build cache", "nothing to prune")
		} else {
			steps = append(steps, cleanupStep{domain.StorageItem{
				Kind: domain.StorageBuildCache, ID: "unused", Name: "Unused build cache", SizeBytes: reclaim,
			}})
		}
	}
	return steps, report
}

// classify sets Eligible and Reason on every item. removing holds container
// IDs that the same plan removes, so their images and volumes count as
// free.
func classify(items []domain.StorageItem, removing map[string]bool) {
	for i := range items {
		it := &items[i]
		it.Eligible, it.Reason = false, ""
		if it.Labels[domain.ProtectLabel] == "true" {
			it.Reason = "protected by the " + domain.ProtectLabel + " label"
			continue
		}
		switch it.Kind {
		case domain.StorageContainer:
			if it.InUse {
				it.Reason = "container is " + it.State
				continue
			}
		case domain.StorageImage, domain.StorageVolume:
			if !it.InUse {
				break
			}
			if len(it.UsedBy) == 0 {
				// The daemon counts a reference we cannot attribute to a
				// container; trust the count.
				it.Reason = "in use"
				continue
			}
			if blocker := firstMissing(it.UsedBy, removing); blocker != "" {
				it.Reason = fmt.Sprintf("used by %d container(s)", len(it.UsedBy))
				continue
			}
		case domain.StorageBuildCache:
			if it.InUse {
				it.Reason = "in use by an active build"
				continue
			}
		}
		it.Eligible = true
	}
}

func firstMissing(ids []string, set map[string]bool) string {
	for _, id := range ids {
		if !set[id] {
			return id
		}
	}
	return ""
}

func dedupe(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func storageKey(kind domain.StorageKind, id string) string { return string(kind) + "\x00" + id }

func kindOrder(k domain.StorageKind) int {
	switch k {
	case domain.StorageContainer:
		return 0
	case domain.StorageImage:
		return 1
	case domain.StorageVolume:
		return 2
	}
	return 3
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
