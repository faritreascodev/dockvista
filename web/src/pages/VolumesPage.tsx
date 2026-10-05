import { useMemo, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { ApiError, createVolume, pruneVolumes, removeVolume } from "../api/client";
import { Button } from "../components/ui/Button";
import { ConfirmDialog } from "../components/ui/ConfirmDialog";
import { Modal } from "../components/ui/Modal";
import { useToast } from "../components/ui/toastContext";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { TableSkeleton } from "../components/Skeleton";
import { useVolumes } from "../hooks/useVolumes";
import { useSearch } from "../hooks/useSearch";
import type { DockerVolume } from "../types/domain";
import { formatBytes, formatRelativeTime } from "../utils/format";

export function VolumesPage() {
  const { data: volumes, error, loading } = useVolumes();
  const { query } = useSearch();
  const [createOpen, setCreateOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<DockerVolume>();
  const toast = useToast();

  const filtered = useMemo(() => {
    if (!query.trim()) return volumes;
    const q = query.trim().toLowerCase();
    return volumes?.filter((v) => v.name.toLowerCase().includes(q));
  }, [volumes, query]);

  const handlePrune = async () => {
    try {
      const result = await pruneVolumes();
      toast.push(
        "success",
        result.deleted === 0
          ? "Nothing to prune."
          : `Removed ${result.deleted} volume${result.deleted === 1 ? "" : "s"}, reclaimed ${formatBytes(result.spaceReclaimedBytes ?? 0)}.`,
      );
    } catch (err) {
      toast.push("error", err instanceof ApiError ? err.message : "Prune failed.");
    }
  };

  return (
    <main className="flex-1 overflow-y-auto px-6 py-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-ink">Volumes</h1>
          <p className="mt-1 text-sm text-ink-muted">Persistent storage managed by the Docker daemon.</p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" onClick={handlePrune}>
            Prune unused
          </Button>
          <Button variant="primary" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" /> Create volume
          </Button>
        </div>
      </div>

      {error && (
        <div className="mt-5">
          <ErrorBanner message={`Failed to load volumes: ${error.message}`} />
        </div>
      )}

      <div className="mt-5 overflow-x-auto rounded-xl border border-edge bg-panel">
        {loading && !volumes ? (
          <TableSkeleton />
        ) : !filtered || filtered.length === 0 ? (
          <EmptyState
            message={query ? "No volumes match your search." : "No volumes found. Create one to persist container data."}
          />
        ) : (
          <table className="w-full min-w-[720px] border-collapse text-left">
            <thead>
              <tr className="border-b border-edge bg-panel-2 text-xs uppercase tracking-wide text-ink-muted">
                <th className="py-2.5 pl-4 font-medium">Name</th>
                <th className="py-2.5 font-medium">Driver</th>
                <th className="py-2.5 font-medium">Mountpoint</th>
                <th className="py-2.5 font-medium">Created</th>
                <th className="py-2.5 pr-4 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-edge">
              {filtered.map((v) => (
                <tr key={v.name} className="text-sm">
                  <td className="py-3 pl-4 font-mono text-xs text-ink">{v.name}</td>
                  <td className="py-3 text-ink-muted">{v.driver}</td>
                  <td className="max-w-xs truncate py-3 text-ink-faint" title={v.mountpoint}>
                    {v.mountpoint}
                  </td>
                  <td className="py-3 text-ink-muted">{formatRelativeTime(v.createdAt)}</td>
                  <td className="py-3 pr-4 text-right">
                    <button
                      onClick={() => setRemoveTarget(v)}
                      className="rounded p-1.5 text-ink-muted transition hover:bg-rose-100 hover:text-rose-600 dark:hover:bg-rose-950/50 dark:hover:text-rose-400"
                      title="Remove volume"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {createOpen && <CreateVolumeModal onClose={() => setCreateOpen(false)} />}

      {removeTarget && (
        <ConfirmDialog
          title="Remove volume"
          description={`This will permanently delete "${removeTarget.name}" and any data it holds. This cannot be undone.`}
          confirmLabel="Remove"
          onConfirm={() => removeVolume(removeTarget.name, true)}
          onClose={() => setRemoveTarget(undefined)}
        />
      )}
    </main>
  );
}

function CreateVolumeModal({ onClose }: { onClose: () => void }) {
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const toast = useToast();

  const handleCreate = async () => {
    setBusy(true);
    setError(undefined);
    try {
      const v = await createVolume(name.trim(), "local");
      toast.push("success", `Created volume "${v.name}".`);
      onClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Create failed.");
      setBusy(false);
    }
  };

  return (
    <Modal
      title="Create volume"
      onClose={onClose}
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button variant="primary" onClick={handleCreate} loading={busy}>
            Create
          </Button>
        </>
      }
    >
      <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="volume-name">
        Name (optional — auto-generated if left blank)
      </label>
      <input
        id="volume-name"
        type="text"
        placeholder="my-data"
        value={name}
        disabled={busy}
        onChange={(e) => setName(e.target.value)}
        className="w-full rounded-lg border border-edge bg-panel-2 px-3 py-2 text-sm text-ink outline-none focus:border-accent disabled:opacity-60"
      />
      {error && (
        <div className="mt-3 rounded-lg border border-rose-300 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-300">
          {error}
        </div>
      )}
    </Modal>
  );
}
