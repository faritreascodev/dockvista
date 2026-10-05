import { useMemo, useState } from "react";
import { Download, Trash2 } from "lucide-react";
import { ApiError, pruneImages, pullImage, removeImage } from "../api/client";
import { Button } from "../components/ui/Button";
import { ConfirmDialog } from "../components/ui/ConfirmDialog";
import { Modal } from "../components/ui/Modal";
import { useToast } from "../components/ui/toastContext";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { TableSkeleton } from "../components/Skeleton";
import { useImages } from "../hooks/useImages";
import { useSearch } from "../hooks/useSearch";
import type { DockerImage } from "../types/domain";
import { formatBytes, formatRelativeTime, shortImageId } from "../utils/format";

export function ImagesPage() {
  const { data: images, error, loading } = useImages();
  const { query } = useSearch();
  const [pullOpen, setPullOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<DockerImage>();
  const toast = useToast();

  const filtered = useMemo(() => {
    if (!query.trim()) return images;
    const q = query.trim().toLowerCase();
    return images?.filter((img) => img.repoTags.some((t) => t.toLowerCase().includes(q)) || img.id.includes(q));
  }, [images, query]);

  const handlePrune = async () => {
    try {
      const result = await pruneImages();
      toast.push(
        "success",
        result.deleted === 0
          ? "Nothing to prune."
          : `Removed ${result.deleted} image${result.deleted === 1 ? "" : "s"}, reclaimed ${formatBytes(result.spaceReclaimedBytes ?? 0)}.`,
      );
    } catch (err) {
      toast.push("error", err instanceof ApiError ? err.message : "Prune failed.");
    }
  };

  return (
    <main className="flex-1 overflow-y-auto px-6 py-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-ink">Images</h1>
          <p className="mt-1 text-sm text-ink-muted">Pull, inspect and remove local Docker images.</p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" onClick={handlePrune}>
            Prune unused
          </Button>
          <Button variant="primary" onClick={() => setPullOpen(true)}>
            <Download className="h-4 w-4" /> Pull image
          </Button>
        </div>
      </div>

      {error && (
        <div className="mt-5">
          <ErrorBanner message={`Failed to load images: ${error.message}`} />
        </div>
      )}

      <div className="mt-5 overflow-x-auto rounded-xl border border-edge bg-panel">
        {loading && !images ? (
          <TableSkeleton />
        ) : !filtered || filtered.length === 0 ? (
          <EmptyState message={query ? "No images match your search." : "No images found. Pull one to get started."} />
        ) : (
          <table className="w-full min-w-[720px] border-collapse text-left">
            <thead>
              <tr className="border-b border-edge bg-panel-2 text-xs uppercase tracking-wide text-ink-muted">
                <th className="py-2.5 pl-4 font-medium">Repository:Tag</th>
                <th className="py-2.5 font-medium">Image ID</th>
                <th className="py-2.5 font-medium">Size</th>
                <th className="py-2.5 font-medium">Created</th>
                <th className="py-2.5 pr-4 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-edge">
              {filtered.map((img) => (
                <tr key={img.id} className="text-sm">
                  <td className="py-3 pl-4 text-ink">
                    {img.repoTags.length > 0 ? (
                      img.repoTags.map((tag) => <div key={tag}>{tag}</div>)
                    ) : (
                      <span className="text-ink-faint">&lt;none&gt;</span>
                    )}
                  </td>
                  <td className="py-3 font-mono text-xs text-ink-muted">{shortImageId(img.id)}</td>
                  <td className="py-3 text-ink-muted">{formatBytes(img.sizeBytes)}</td>
                  <td className="py-3 text-ink-muted">{formatRelativeTime(img.createdAt)}</td>
                  <td className="py-3 pr-4 text-right">
                    <button
                      onClick={() => setRemoveTarget(img)}
                      className="rounded p-1.5 text-ink-muted transition hover:bg-rose-100 hover:text-rose-600 dark:hover:bg-rose-950/50 dark:hover:text-rose-400"
                      title="Remove image"
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

      {pullOpen && <PullImageModal onClose={() => setPullOpen(false)} />}

      {removeTarget && (
        <ConfirmDialog
          title="Remove image"
          description={`This will permanently remove ${removeTarget.repoTags[0] ?? shortImageId(removeTarget.id)}. Containers using it won't be affected until removed.`}
          confirmLabel="Remove"
          onConfirm={() => removeImage(removeTarget.id, true)}
          onClose={() => setRemoveTarget(undefined)}
        />
      )}
    </main>
  );
}

function PullImageModal({ onClose }: { onClose: () => void }) {
  const [reference, setReference] = useState("");
  const [log, setLog] = useState<string[]>([]);
  const [pulling, setPulling] = useState(false);
  const [error, setError] = useState<string>();
  const toast = useToast();

  const handlePull = async () => {
    setPulling(true);
    setError(undefined);
    setLog([]);
    try {
      await pullImage(reference, (line) => {
        setLog((prev) => [...prev.slice(-49), line]);
      });
      toast.push("success", `Pulled ${reference}.`);
      onClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Pull failed.");
    } finally {
      setPulling(false);
    }
  };

  return (
    <Modal
      title="Pull image"
      onClose={onClose}
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={pulling}>
            {pulling ? "Close" : "Cancel"}
          </Button>
          <Button variant="primary" onClick={handlePull} loading={pulling} disabled={!reference.trim()}>
            Pull
          </Button>
        </>
      }
    >
      <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="pull-reference">
        Image reference
      </label>
      <input
        id="pull-reference"
        type="text"
        placeholder="nginx:latest"
        value={reference}
        disabled={pulling}
        onChange={(e) => setReference(e.target.value)}
        className="w-full rounded-lg border border-edge bg-panel-2 px-3 py-2 text-sm text-ink outline-none focus:border-accent disabled:opacity-60"
      />

      {log.length > 0 && (
        <div className="mt-3 max-h-40 overflow-y-auto rounded-lg border border-edge bg-panel-2 p-2 font-mono text-xs text-ink-muted">
          {log.map((line, i) => (
            <div key={i} className="truncate">
              {line}
            </div>
          ))}
        </div>
      )}

      {error && (
        <div className="mt-3 rounded-lg border border-rose-300 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-300">
          {error}
        </div>
      )}
    </Modal>
  );
}
