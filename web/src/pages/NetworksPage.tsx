import { useMemo, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { ApiError, createNetwork, pruneNetworks, removeNetwork } from "../api/client";
import { Button } from "../components/ui/Button";
import { ConfirmDialog } from "../components/ui/ConfirmDialog";
import { Modal } from "../components/ui/Modal";
import { useToast } from "../components/ui/Toast";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { TableSkeleton } from "../components/Skeleton";
import { useNetworks } from "../hooks/useNetworks";
import { useSearch } from "../hooks/useSearchContext";
import type { DockerNetwork } from "../types/domain";
import { formatRelativeTime, shortId } from "../utils/format";

export function NetworksPage() {
  const { data: networks, error, loading } = useNetworks();
  const { query } = useSearch();
  const [createOpen, setCreateOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<DockerNetwork>();
  const toast = useToast();

  const filtered = useMemo(() => {
    if (!query.trim()) return networks;
    const q = query.trim().toLowerCase();
    return networks?.filter((n) => n.name.toLowerCase().includes(q));
  }, [networks, query]);

  const handlePrune = async () => {
    try {
      const result = await pruneNetworks();
      toast.push(
        "success",
        result.deleted === 0 ? "Nothing to prune." : `Removed ${result.deleted} network${result.deleted === 1 ? "" : "s"}.`,
      );
    } catch (err) {
      toast.push("error", err instanceof ApiError ? err.message : "Prune failed.");
    }
  };

  return (
    <main className="flex-1 overflow-y-auto px-6 py-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-ink">Networks</h1>
          <p className="mt-1 text-sm text-ink-muted">Bridge, overlay and custom Docker networks.</p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" onClick={handlePrune}>
            Prune unused
          </Button>
          <Button variant="primary" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" /> Create network
          </Button>
        </div>
      </div>

      {error && (
        <div className="mt-5">
          <ErrorBanner message={`Failed to load networks: ${error.message}`} />
        </div>
      )}

      <div className="mt-5 overflow-x-auto rounded-xl border border-edge bg-panel">
        {loading && !networks ? (
          <TableSkeleton />
        ) : !filtered || filtered.length === 0 ? (
          <EmptyState message={query ? "No networks match your search." : "No networks found."} />
        ) : (
          <table className="w-full min-w-[720px] border-collapse text-left">
            <thead>
              <tr className="border-b border-edge bg-panel-2 text-xs uppercase tracking-wide text-ink-muted">
                <th className="py-2.5 pl-4 font-medium">Name</th>
                <th className="py-2.5 font-medium">Driver</th>
                <th className="py-2.5 font-medium">Scope</th>
                <th className="py-2.5 font-medium">Containers</th>
                <th className="py-2.5 font-medium">Created</th>
                <th className="py-2.5 pr-4 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-edge">
              {filtered.map((n) => {
                const isBuiltin = ["bridge", "host", "none"].includes(n.name);
                return (
                  <tr key={n.id} className="text-sm">
                    <td className="py-3 pl-4 text-ink">
                      {n.name}
                      <span className="ml-1.5 font-mono text-xs text-ink-faint">{shortId(n.id)}</span>
                    </td>
                    <td className="py-3 text-ink-muted">{n.driver}</td>
                    <td className="py-3 text-ink-muted">{n.scope}</td>
                    <td className="py-3 text-ink-muted">{Object.keys(n.containers ?? {}).length}</td>
                    <td className="py-3 text-ink-muted">{formatRelativeTime(n.createdAt)}</td>
                    <td className="py-3 pr-4 text-right">
                      <button
                        onClick={() => setRemoveTarget(n)}
                        disabled={isBuiltin}
                        title={isBuiltin ? "Built-in networks can't be removed" : "Remove network"}
                        className="rounded p-1.5 text-ink-muted transition hover:bg-rose-100 hover:text-rose-600 disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:bg-transparent disabled:hover:text-ink-muted dark:hover:bg-rose-950/50 dark:hover:text-rose-400"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      {createOpen && <CreateNetworkModal onClose={() => setCreateOpen(false)} />}

      {removeTarget && (
        <ConfirmDialog
          title="Remove network"
          description={`This will permanently delete the "${removeTarget.name}" network.`}
          confirmLabel="Remove"
          onConfirm={() => removeNetwork(removeTarget.id)}
          onClose={() => setRemoveTarget(undefined)}
        />
      )}
    </main>
  );
}

function CreateNetworkModal({ onClose }: { onClose: () => void }) {
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const toast = useToast();

  const handleCreate = async () => {
    setBusy(true);
    setError(undefined);
    try {
      const n = await createNetwork(name.trim(), "bridge");
      toast.push("success", `Created network "${n.name}".`);
      onClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Create failed.");
      setBusy(false);
    }
  };

  return (
    <Modal
      title="Create network"
      onClose={onClose}
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button variant="primary" onClick={handleCreate} loading={busy} disabled={!name.trim()}>
            Create
          </Button>
        </>
      }
    >
      <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="network-name">
        Name
      </label>
      <input
        id="network-name"
        type="text"
        placeholder="my-network"
        value={name}
        disabled={busy}
        onChange={(e) => setName(e.target.value)}
        className="w-full rounded-lg border border-edge bg-panel-2 px-3 py-2 text-sm text-ink outline-none focus:border-accent disabled:opacity-60"
      />
      <p className="mt-1 text-xs text-ink-faint">Driver: bridge (default).</p>
      {error && (
        <div className="mt-3 rounded-lg border border-rose-300 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-300">
          {error}
        </div>
      )}
    </Modal>
  );
}
