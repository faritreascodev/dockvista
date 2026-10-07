package service

import (
	"context"
	"errors"
	"testing"

	"dockvista/internal/core/domain"
)

type fakeStorage struct {
	items         []domain.StorageItem
	removed       []string
	failContainer string
	pruned        bool
	imageForce    map[string]bool
}

func (f *fakeStorage) StorageInventory(context.Context) (domain.StorageInventory, error) {
	return domain.StorageInventory{Items: append([]domain.StorageItem(nil), f.items...)}, nil
}

func (f *fakeStorage) RemoveContainer(_ context.Context, id string, force bool) error {
	if force {
		return errors.New("cleanup must never force a container")
	}
	if id == f.failContainer {
		return errors.New("daemon refused")
	}
	f.removed = append(f.removed, "container:"+id)
	return nil
}

func (f *fakeStorage) RemoveImage(_ context.Context, id string, force bool) error {
	if f.imageForce == nil {
		f.imageForce = map[string]bool{}
	}
	f.imageForce[id] = force
	f.removed = append(f.removed, "image:"+id)
	return nil
}

func (f *fakeStorage) RemoveVolume(_ context.Context, name string, force bool) error {
	if force {
		return errors.New("cleanup must never force a volume")
	}
	f.removed = append(f.removed, "volume:"+name)
	return nil
}

func (f *fakeStorage) PruneBuildCache(context.Context) (uint64, error) {
	f.pruned = true
	return 4096, nil
}

func sampleInventory() []domain.StorageItem {
	return []domain.StorageItem{
		{Kind: domain.StorageContainer, ID: "run", Name: "api", State: "running", InUse: true, SizeBytes: 10},
		{Kind: domain.StorageContainer, ID: "old", Name: "old-job", State: "exited", SizeBytes: 20},
		{Kind: domain.StorageContainer, ID: "keep", Name: "keep-me", State: "exited",
			Labels: map[string]string{domain.ProtectLabel: "true"}},
		{Kind: domain.StorageImage, ID: "img-run", Tags: []string{"api:latest"}, UsedBy: []string{"run"}, InUse: true, SizeBytes: 100},
		{Kind: domain.StorageImage, ID: "img-old", Tags: []string{"job:1", "job:latest"}, UsedBy: []string{"old"}, InUse: true, SizeBytes: 200},
		{Kind: domain.StorageImage, ID: "img-dangling", SizeBytes: 300},
		{Kind: domain.StorageVolume, ID: "pgdata", UsedBy: []string{"run"}, InUse: true, SizeBytes: 1000},
		{Kind: domain.StorageVolume, ID: "scratch", UsedBy: []string{"old"}, InUse: true, SizeBytes: 50},
		{Kind: domain.StorageVolume, ID: "orphan", SizeBytes: 70},
		{Kind: domain.StorageBuildCache, ID: "c1", InUse: true, SizeBytes: 5},
		{Kind: domain.StorageBuildCache, ID: "c2", SizeBytes: 8},
	}
}

func resultsByID(r domain.CleanupReport) map[string]domain.CleanupResult {
	out := map[string]domain.CleanupResult{}
	for _, res := range r.Results {
		out[res.ID] = res
	}
	return out
}

func TestInventory_ClassifiesSafety(t *testing.T) {
	svc := NewStorageService(&fakeStorage{items: sampleInventory()}, nil)
	inv, err := svc.Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"run": false, "old": true, "keep": false,
		"img-run": false, "img-old": false, "img-dangling": true,
		"pgdata": false, "scratch": false, "orphan": true,
		"c1": false, "c2": true,
	}
	for _, it := range inv.Items {
		if it.Eligible != want[it.ID] {
			t.Errorf("%s: eligible = %v (%s), want %v", it.ID, it.Eligible, it.Reason, want[it.ID])
		}
		if !it.Eligible && it.Reason == "" {
			t.Errorf("%s: ineligible without a reason", it.ID)
		}
	}
}

func TestCleanup_RefusesEverythingUnsafe(t *testing.T) {
	fake := &fakeStorage{items: sampleInventory()}
	svc := NewStorageService(fake, nil)
	report, err := svc.Cleanup(context.Background(), domain.CleanupPlan{
		Containers: []string{"run", "keep", "ghost"},
		Images:     []string{"img-run", "img-old"},
		Volumes:    []string{"pgdata", "scratch"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.removed) != 0 {
		t.Fatalf("removed %v; nothing in this plan is safe", fake.removed)
	}
	for id, res := range resultsByID(report) {
		if res.Status != domain.CleanupSkipped || res.Message == "" {
			t.Errorf("%s: status %s %q, want skipped with a reason", id, res.Status, res.Message)
		}
	}
}

func TestCleanup_CascadesThroughContainersInThePlan(t *testing.T) {
	fake := &fakeStorage{items: sampleInventory()}
	changed := false
	svc := NewStorageService(fake, func() { changed = true })
	report, err := svc.Cleanup(context.Background(), domain.CleanupPlan{
		// Listed out of order on purpose: execution must still remove the
		// container before the image and volume it holds.
		Volumes:    []string{"scratch", "orphan"},
		Images:     []string{"img-old", "img-dangling"},
		Containers: []string{"old"},
		BuildCache: true,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"container:old", "image:img-old", "image:img-dangling", "volume:scratch", "volume:orphan"}
	if len(fake.removed) != len(wantOrder) {
		t.Fatalf("removed %v, want %v", fake.removed, wantOrder)
	}
	for i := range wantOrder {
		if fake.removed[i] != wantOrder[i] {
			t.Fatalf("removed %v, want order %v", fake.removed, wantOrder)
		}
	}
	if !fake.imageForce["img-old"] || fake.imageForce["img-dangling"] {
		t.Fatalf("force flags %v: only the multi-tag image needs force", fake.imageForce)
	}
	if !fake.pruned || !changed {
		t.Fatalf("pruned=%v changed=%v, want both", fake.pruned, changed)
	}
	if report.FreedBytes != 20+200+300+50+70+4096 {
		t.Fatalf("freed = %d", report.FreedBytes)
	}
}

func TestCleanup_KeepsDependentsWhenTheirContainerFails(t *testing.T) {
	fake := &fakeStorage{items: sampleInventory(), failContainer: "old"}
	svc := NewStorageService(fake, nil)
	report, err := svc.Cleanup(context.Background(), domain.CleanupPlan{
		Containers: []string{"old"},
		Images:     []string{"img-old"},
		Volumes:    []string{"scratch"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.removed) != 0 {
		t.Fatalf("removed %v after the container they depend on failed", fake.removed)
	}
	got := resultsByID(report)
	if got["old"].Status != domain.CleanupFailed {
		t.Fatalf("container status %s, want failed", got["old"].Status)
	}
	for _, id := range []string{"img-old", "scratch"} {
		if got[id].Status != domain.CleanupSkipped {
			t.Fatalf("%s status %s, want skipped", id, got[id].Status)
		}
	}
}

func TestCleanup_DryRunTouchesNothing(t *testing.T) {
	fake := &fakeStorage{items: sampleInventory()}
	svc := NewStorageService(fake, nil)
	report, err := svc.Cleanup(context.Background(), domain.CleanupPlan{
		Containers: []string{"old"}, Images: []string{"img-old"}, BuildCache: true,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.removed) != 0 || fake.pruned {
		t.Fatalf("dry run removed %v pruned=%v", fake.removed, fake.pruned)
	}
	if !report.DryRun || report.FreedBytes != 20+200+8 {
		t.Fatalf("report = %+v", report)
	}
	for _, res := range report.Results {
		if res.Status != domain.CleanupPlanned {
			t.Fatalf("%s: status %s, want planned", res.ID, res.Status)
		}
	}
}

func TestCleanup_RejectsEmptyAndOversizedPlans(t *testing.T) {
	svc := NewStorageService(&fakeStorage{}, nil)
	huge := make([]string, maxCleanupItems+1)
	for i := range huge {
		huge[i] = "x"
	}
	for _, plan := range []domain.CleanupPlan{{}, {Volumes: huge}} {
		if _, err := svc.Cleanup(context.Background(), plan, true); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("plan of %d items: err = %v, want ErrInvalidInput", plan.Size(), err)
		}
	}
}
