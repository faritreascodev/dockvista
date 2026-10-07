import { useMemo, useState } from "react";
import { Eraser, Info, Shield, Trash2, X } from "lucide-react";
import { previewCleanup, runCleanup } from "../api/client";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/Skeleton";
import { StatCard } from "../components/StatCard";
import { Button } from "../components/ui/Button";
import { Modal } from "../components/ui/Modal";
import { Segmented } from "../components/ui/Segmented";
import { Chip, DataTable, TableFrame } from "../components/ui/Table";
import { useToast } from "../components/ui/toastContext";
import { useSearch } from "../hooks/useSearch";
import { useSession } from "../hooks/useSession";
import { useStorageInventory } from "../hooks/useStorage";
import type { CleanupPlan, CleanupReport, StorageItem, StorageKind } from "../types/domain";
import { formatBytes, formatRelativeIso } from "../utils/format";

const PROTECT = "dockvista.protect";

type Tab = "all" | StorageKind;

function emptyPlan(): CleanupPlan {
  return { containers: [], images: [], volumes: [], buildCache: false };
}

function planSize(plan: CleanupPlan): number {
  return plan.containers.length + plan.images.length + plan.volumes.length + (plan.buildCache ? 1 : 0);
}

function isSafe(item: StorageItem): boolean {
  if (!item.eligible) return false;
  if (item.kind === "volume") return false;
  if (item.kind === "image") return item.tags.length === 0;
  return item.kind === "container" || item.kind === "buildcache";
}

function matchesQuery(item: StorageItem, q: string): boolean {
  if (!q) return true;
  const hay = `${item.name} ${item.id} ${item.detail ?? ""} ${item.project ?? ""} ${item.tags.join(" ")}`.toLowerCase();
  return hay.includes(q);
}

export function StoragePage() {
  const [nonce, setNonce] = useState(0);
  const { data, error, loading } = useStorageInventory(nonce);
  const { query } = useSearch();
  const { readOnly } = useSession();
  const toast = useToast();

  const [tab, setTab] = useState<Tab>("all");
  const [eligibleOnly, setEligibleOnly] = useState(true);
  const [plan, setPlan] = useState<CleanupPlan>(emptyPlan);
  const [review, setReview] = useState<CleanupReport>();
  const [previewing, setPreviewing] = useState(false);

  const items = useMemo(() => data?.items ?? [], [data]);
  const q = query.trim().toLowerCase();

  const rows = useMemo(() => {
    return items.filter((it) => {
      if (it.kind === "buildcache") return false;
      if (tab !== "all" && it.kind !== tab) return false;
      if (eligibleOnly && !it.eligible) return false;
      return matchesQuery(it, q);
    });
  }, [items, tab, eligibleOnly, q]);

  const cache = useMemo(() => items.filter((it) => it.kind === "buildcache"), [items]);
  const cacheEligible = cache.filter((it) => it.eligible && !it.shared);
  const cacheBytes = cacheEligible.reduce((s, it) => s + it.sizeBytes, 0);

  const eligible = items.filter((it) => it.eligible);
  const reclaimable = eligible
    .filter((it) => it.kind !== "buildcache" || !it.shared)
    .reduce((s, it) => s + it.sizeBytes, 0);
  const used =
    (data?.usage.images.sizeBytes ?? 0) +
    (data?.usage.containers.sizeBytes ?? 0) +
    (data?.usage.volumes.sizeBytes ?? 0) +
    (data?.usage.buildCache.sizeBytes ?? 0);

  const selectedCount = planSize(plan);
  const selectedBytes = useMemo(() => {
    let n = 0;
    const ids = new Set([...plan.containers, ...plan.images, ...plan.volumes]);
    for (const it of items) {
      if (ids.has(it.id) && (it.kind === "container" || it.kind === "image" || it.kind === "volume")) {
        n += it.sizeBytes;
      }
    }
    if (plan.buildCache) n += cacheBytes;
    return n;
  }, [items, plan, cacheBytes]);

  const selected = (kind: StorageKind, id: string) => {
    if (kind === "container") return plan.containers.includes(id);
    if (kind === "image") return plan.images.includes(id);
    if (kind === "volume") return plan.volumes.includes(id);
    return plan.buildCache;
  };

  const toggle = (item: StorageItem) => {
    if (!item.eligible) return;
    setPlan((current) => {
      const next = { ...current, containers: [...current.containers], images: [...current.images], volumes: [...current.volumes] };
      const flip = (list: string[]) => {
        const i = list.indexOf(item.id);
        if (i === -1) list.push(item.id);
        else list.splice(i, 1);
      };
      if (item.kind === "container") flip(next.containers);
      else if (item.kind === "image") flip(next.images);
      else if (item.kind === "volume") flip(next.volumes);
      return next;
    });
  };

  const applyPreset = (which: "safe" | "images" | "volumes") => {
    const next = emptyPlan();
    next.buildCache = which === "safe" && cacheEligible.length > 0;
    for (const it of eligible) {
      if (which === "safe" && !isSafe(it)) continue;
      if (which === "images" && it.kind !== "image") continue;
      if (which === "volumes" && it.kind !== "volume") continue;
      if (it.kind === "container") next.containers.push(it.id);
      else if (it.kind === "image") next.images.push(it.id);
      else if (it.kind === "volume") next.volumes.push(it.id);
    }
    const n = planSize(next);
    if (n === 0) {
      toast.push(
        "info",
        which === "safe"
          ? "Nothing is safe to reclaim right now: no stopped containers, dangling images, or unused build cache."
          : which === "images"
            ? "No unused images. Everything is still referenced by a container."
            : "No unused volumes. Everything is still mounted by a container.",
      );
      return;
    }
    setPlan(next);
    let bytes = cacheEligible.length && next.buildCache ? cacheBytes : 0;
    const picked = new Set([...next.containers, ...next.images, ...next.volumes]);
    for (const it of items) {
      if (picked.has(it.id) && it.kind !== "buildcache") bytes += it.sizeBytes;
    }
    toast.push("success", `Queued ${n} object${n === 1 ? "" : "s"} · ${formatBytes(bytes)}. Review to delete, or clear the selection.`);
  };

  const openReview = async () => {
    if (selectedCount === 0) return;
    setPreviewing(true);
    try {
      setReview(await previewCleanup(plan));
    } catch (err) {
      toast.push("error", err instanceof Error ? err.message : "Could not preview cleanup.");
    } finally {
      setPreviewing(false);
    }
  };

  return (
    <main className="flex-1 overflow-y-auto px-4 py-6 md:px-8">
      <PageHeader
        kicker="Resources"
        title="Storage"
        description="See what occupies disk and remove only what you pick. Running containers, in-use images, and anything labelled dockvista.protect=true stay put."
        actions={
          !readOnly && (
            <>
              <Button onClick={() => applyPreset("safe")}>Safe reclaim</Button>
              <Button onClick={() => applyPreset("images")}>Unused images</Button>
              <Button onClick={() => applyPreset("volumes")}>Unused volumes</Button>
              {selectedCount > 0 && (
                <Button onClick={() => setPlan(emptyPlan())}>
                  <X className="h-4 w-4" /> Clear selection
                </Button>
              )}
              <Button variant="danger" disabled={selectedCount === 0} onClick={() => void openReview()} loading={previewing}>
                <Eraser className="h-4 w-4" /> Review {selectedCount > 0 ? selectedCount : ""}
              </Button>
            </>
          )
        }
      />

      {error && (
        <div className="mt-5">
          <ErrorBanner message={`Failed to load storage: ${error.message}`} />
        </div>
      )}

      <div className="mt-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard label="On disk" value={formatBytes(used)} hint="inside the Docker engine, not Windows C:" />
        <StatCard
          label="Reclaimable"
          value={formatBytes(reclaimable)}
          hint={`${eligible.length} unused objects · engine disk only`}
          tone="warn"
        />
        <StatCard label="Selected" value={formatBytes(selectedBytes)} hint={selectedCount === 0 ? "nothing queued" : `${selectedCount} in the review plan`} tone="accent" />
        <StatCard
          label="Build cache"
          value={formatBytes(data?.usage.buildCache.sizeBytes ?? 0)}
          hint={`${cacheEligible.length} unused records · ${formatBytes(cacheBytes)} freeable`}
        />
      </div>

      <div className="mt-6 flex flex-wrap items-center gap-3">
        <Segmented
          value={tab}
          onChange={setTab}
          options={[
            { value: "all", label: "All" },
            { value: "container", label: "Containers", count: items.filter((i) => i.kind === "container").length },
            { value: "image", label: "Images", count: items.filter((i) => i.kind === "image").length },
            { value: "volume", label: "Volumes", count: items.filter((i) => i.kind === "volume").length },
            { value: "buildcache", label: "Cache", count: cache.length },
          ]}
        />
        <label className="flex items-center gap-2 text-xs text-ink-muted">
          <input type="checkbox" checked={eligibleOnly} onChange={(e) => setEligibleOnly(e.target.checked)} className="accent-[rgb(var(--color-accent))]" />
          Removable only
        </label>
      </div>

      <div className="mt-4 rounded-lg border border-info/30 bg-info/10 px-4 py-3 text-sm text-ink" role="note">
        <p className="flex items-start gap-2">
          <Info className="mt-0.5 h-4 w-4 shrink-0 text-info" />
          <span>
            <span className="font-medium">Reclaimable is Docker's disk, not Windows C:.</span> Docker Desktop stores
            layers in a WSL2 virtual disk. A prune frees space inside that disk; Explorer does not change until you
            compact it (quit Docker Desktop, then compact{" "}
            <span className="font-mono text-xs">docker_data.vhdx</span>, or Docker Desktop → Settings → Resources). On a
            native Linux daemon, prune does free the host disk.
          </span>
        </p>
      </div>

      {(tab === "all" || tab === "buildcache") && cache.length > 0 && (
        <div className="mt-4 flex items-center gap-3 rounded-lg border border-edge bg-panel px-4 py-3">
          <input
            type="checkbox"
            disabled={readOnly || cacheEligible.length === 0}
            checked={plan.buildCache}
            onChange={() => setPlan((p) => ({ ...p, buildCache: !p.buildCache }))}
            className="accent-[rgb(var(--color-accent))]"
            aria-label="Select unused build cache"
          />
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium text-ink">Unused build cache</p>
            <p className="text-xs text-ink-muted">
              BuildKit records not in an active build. Pruned as a unit — {cacheEligible.length} of {cache.length} records, {formatBytes(cacheBytes)}.
            </p>
          </div>
        </div>
      )}

      {tab !== "buildcache" && (
        <div className="mt-4">
          <TableFrame>
            {loading && !data ? (
              <TableSkeleton />
            ) : rows.length === 0 ? (
              <EmptyState
                icon={Trash2}
                message={
                  q
                    ? "Nothing matches this filter."
                    : eligibleOnly
                      ? "Nothing removable here. Turn off “Removable only” to see in-use objects, or try another preset."
                      : "Nothing in this category."
                }
              />
            ) : (
              <DataTable columns={["", "Name", "Kind", "Size", "Why", ""]}>
                {rows.map((it) => {
                  const protectedItem = it.labels?.[PROTECT] === "true";
                  return (
                    <tr key={`${it.kind}:${it.id}`} className={`transition-colors hover:bg-panel-2/60 ${it.eligible ? "" : "opacity-70"}`}>
                      <td className="w-10 py-3 pl-4">
                        <input
                          type="checkbox"
                          disabled={readOnly || !it.eligible}
                          checked={selected(it.kind, it.id)}
                          onChange={() => toggle(it)}
                          aria-label={`Select ${it.name}`}
                          className="accent-[rgb(var(--color-accent))]"
                        />
                      </td>
                      <td className="max-w-[340px] py-3 pr-4">
                        <p className="truncate font-mono text-xs text-ink" title={it.name}>
                          {it.name}
                        </p>
                        <p className="truncate font-mono text-[11px] text-ink-faint">
                          {it.detail ?? it.id.slice(0, 12)}
                          {it.project ? ` · ${it.project}` : ""}
                        </p>
                      </td>
                      <td className="py-3 pr-4">
                        <div className="flex flex-wrap gap-1">
                          <Chip>{it.kind}</Chip>
                          {it.state && <Chip tone={it.state === "running" ? "ok" : "muted"}>{it.state}</Chip>}
                          {protectedItem && (
                            <Chip tone="warn">
                              <span className="inline-flex items-center gap-1">
                                <Shield className="h-3 w-3" /> protected
                              </span>
                            </Chip>
                          )}
                          {it.kind === "image" && it.tags.length === 0 && <Chip tone="accent">dangling</Chip>}
                        </div>
                      </td>
                      <td className="whitespace-nowrap py-3 pr-4 font-mono text-xs tabular-nums text-ink">
                        {it.sizeKnown ? formatBytes(it.sizeBytes) : "—"}
                      </td>
                      <td className="max-w-[220px] truncate py-3 pr-4 text-xs text-ink-muted" title={it.reason}>
                        {it.eligible ? "can remove" : (it.reason ?? "kept")}
                      </td>
                      <td className="whitespace-nowrap py-3 pr-4 text-right text-[11px] text-ink-faint">{formatRelativeIso(it.createdAt)}</td>
                    </tr>
                  );
                })}
              </DataTable>
            )}
          </TableFrame>
        </div>
      )}

      {review && (
        <CleanupReview
          report={review}
          hasVolumes={plan.volumes.length > 0}
          onClose={() => setReview(undefined)}
          onConfirm={async () => {
            const result = await runCleanup(plan);
            const removed = result.results.filter((r) => r.status === "removed").length;
            const failed = result.results.filter((r) => r.status === "failed").length;
            toast.push(
              failed > 0 ? "error" : "success",
              failed > 0
                ? `Removed ${removed}, ${failed} failed. Engine reported ${formatBytes(result.freedBytes)}.`
                : `Removed ${removed} object${removed === 1 ? "" : "s"} inside the Docker disk (${formatBytes(result.freedBytes)}). Windows C: does not grow until that virtual disk is compacted.`,
            );
            setPlan(emptyPlan());
            setNonce((n) => n + 1);
          }}
        />
      )}
    </main>
  );
}

function CleanupReview({
  report,
  hasVolumes,
  onClose,
  onConfirm,
}: {
  report: CleanupReport;
  hasVolumes: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
}) {
  const [typed, setTyped] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const planned = report.results.filter((r) => r.status === "planned");
  const skipped = report.results.filter((r) => r.status === "skipped");
  const phrase = hasVolumes ? "DELETE VOLUMES" : "DELETE";
  const ready = typed.trim() === phrase && planned.length > 0;

  const confirm = async () => {
    if (!ready) return;
    setBusy(true);
    setError(undefined);
    try {
      await onConfirm();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Cleanup failed.");
      setBusy(false);
    }
  };

  return (
    <Modal
      title="Review cleanup"
      size="xl"
      onClose={onClose}
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button variant="danger" onClick={() => void confirm()} disabled={!ready} loading={busy}>
            Delete {planned.length} objects
          </Button>
        </>
      }
    >
      <p className="text-sm text-ink-muted">
        The engine re-checked every item just now. Only the planned rows will be removed. This cannot be undone.
        Estimated free <span className="font-medium text-ink">inside the Docker engine</span>:{" "}
        <span className="font-mono text-ink">{formatBytes(report.freedBytes)}</span>. This does not automatically
        increase free space on Windows C:.
      </p>
      {hasVolumes && (
        <p className="mt-2 rounded-md border border-warn/30 bg-warn/10 px-3 py-2 text-sm text-warn">
          This plan includes volumes. Named volumes hold databases and uploads — deleting them destroys that data.
        </p>
      )}
      <ul className="mt-4 max-h-64 overflow-y-auto rounded-md border border-edge divide-y divide-edge text-sm">
        {report.results.map((r) => (
          <li key={`${r.kind}:${r.id}`} className="flex items-start justify-between gap-3 px-3 py-2">
            <div className="min-w-0">
              <p className="truncate font-mono text-xs text-ink">{r.name}</p>
              {r.message && <p className="text-[11px] text-ink-muted">{r.message}</p>}
            </div>
            <span className={`shrink-0 font-mono text-[11px] ${r.status === "planned" ? "text-warn" : "text-ink-faint"}`}>
              {r.status}
              {r.freedBytes > 0 ? ` · ${formatBytes(r.freedBytes)}` : ""}
            </span>
          </li>
        ))}
      </ul>
      {skipped.length > 0 && (
        <p className="mt-2 text-xs text-ink-faint">{skipped.length} skipped because they are in use, protected, or gone.</p>
      )}
      <label className="mt-4 block text-xs text-ink-muted">
        Type <span className="font-mono text-ink">{phrase}</span> to confirm
        <input
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
          disabled={busy}
          autoComplete="off"
          spellCheck={false}
          className="field mt-1.5 font-mono"
        />
      </label>
      {error && <p className="mt-3 text-sm text-bad">{error}</p>}
    </Modal>
  );
}
