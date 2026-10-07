import { useEffect, useMemo, useState } from "react";
import { Download, Eraser, Image as ImageIcon } from "lucide-react";
import { ApiError, getImageHistory, pruneImages, pullImage, removeImage } from "../api/client";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/Skeleton";
import { Button } from "../components/ui/Button";
import { ConfirmDialog } from "../components/ui/ConfirmDialog";
import { FieldLabel, FormError } from "../components/ui/Form";
import { Modal } from "../components/ui/Modal";
import { Chip, DataTable, RemoveButton, TableFrame } from "../components/ui/Table";
import { useToast } from "../components/ui/toastContext";
import { useImages } from "../hooks/useImages";
import { useSearch } from "../hooks/useSearch";
import { useSession } from "../hooks/useSession";
import type { DockerImage, ImageLayer } from "../types/domain";
import { formatBytes, formatRelativeTime, shortImageId } from "../utils/format";

function splitTag(tag: string): [string, string] {
  const at = tag.lastIndexOf(":");
  return at > tag.lastIndexOf("/") ? [tag.slice(0, at), tag.slice(at + 1)] : [tag, "latest"];
}

export function ImagesPage() {
  const { data: images, error, loading } = useImages();
  const { query } = useSearch();
  const { readOnly } = useSession();
  const [pullOpen, setPullOpen] = useState(false);
  const [pruneOpen, setPruneOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<DockerImage>();
  const [historyTarget, setHistoryTarget] = useState<DockerImage>();
  const toast = useToast();

  const filtered = useMemo(() => {
    if (!query.trim()) return images;
    const q = query.trim().toLowerCase();
    return images?.filter((img) => img.repoTags.some((t) => t.toLowerCase().includes(q)) || img.id.includes(q));
  }, [images, query]);

  const totalSize = useMemo(() => (images ?? []).reduce((sum, i) => sum + i.sizeBytes, 0), [images]);

  return (
    <main className="flex-1 overflow-y-auto px-4 py-6 md:px-8">
      <PageHeader
        kicker="Resources"
        title="Images"
        description={
          images ? (
            <>
              <span className="font-mono text-ink">{images.length}</span> images ·{" "}
              <span className="font-mono text-ink">{formatBytes(totalSize)}</span> on disk
            </>
          ) : (
            "Local images available to this engine."
          )
        }
        actions={
          !readOnly && (
            <>
              <Button onClick={() => setPruneOpen(true)}>
                <Eraser className="h-4 w-4" /> Prune dangling
              </Button>
              <Button variant="primary" onClick={() => setPullOpen(true)}>
                <Download className="h-4 w-4" /> Pull image
              </Button>
            </>
          )
        }
      />

      {error && (
        <div className="mt-5">
          <ErrorBanner message={`Failed to load images: ${error.message}`} />
        </div>
      )}

      <div className="mt-6">
        <TableFrame>
          {loading && !images ? (
            <TableSkeleton />
          ) : !filtered || filtered.length === 0 ? (
            <EmptyState
              icon={ImageIcon}
              message={query ? "No images match your filter." : "No images yet. Pull one to get started."}
            />
          ) : (
            <DataTable columns={["Repository", "Tag", "Image ID", "Size", "Created", ""]}>
              {filtered.map((img) => {
                const tags = img.repoTags.length ? img.repoTags : ["<none>:<none>"];
                const [repo] = splitTag(tags[0] ?? "");
                return (
                  <tr key={img.id} className="transition-colors hover:bg-panel-2/60">
                    <td className="py-3 pl-4 pr-4">
                      <div className="flex items-center gap-2">
                        <span className={repo === "<none>" ? "text-ink-faint" : "font-medium text-ink"}>{repo}</span>
                        {img.containers > 0 && <Chip tone="ok">in use · {img.containers}</Chip>}
                      </div>
                    </td>
                    <td className="py-3 pr-4">
                      <div className="flex flex-wrap gap-1">
                        {tags.map((t) => (
                          <Chip key={t} tone={splitTag(t)[1] === "latest" ? "accent" : "muted"}>
                            {splitTag(t)[1]}
                          </Chip>
                        ))}
                      </div>
                    </td>
                    <td className="py-3 pr-4 font-mono text-xs text-ink-muted">{shortImageId(img.id)}</td>
                    <td className="py-3 pr-4 font-mono text-xs text-ink">{formatBytes(img.sizeBytes)}</td>
                    <td className="whitespace-nowrap py-3 pr-4 text-xs text-ink-muted">{formatRelativeTime(img.createdAt)}</td>
                    <td className="py-3 pr-4 text-right">
                      <div className="flex justify-end gap-2">
                        <button
                          type="button"
                          className="text-xs text-accent hover:underline"
                          onClick={() => setHistoryTarget(img)}
                        >
                          History
                        </button>
                        {!readOnly && <RemoveButton title="Remove image" onClick={() => setRemoveTarget(img)} />}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </DataTable>
          )}
        </TableFrame>
      </div>

      {historyTarget && <HistoryModal image={historyTarget} onClose={() => setHistoryTarget(undefined)} />}

      {pullOpen && <PullImageModal onClose={() => setPullOpen(false)} />}

      {pruneOpen && (
        <ConfirmDialog
          title="Prune dangling images"
          description="Removes untagged images that no container uses. Tagged images are kept."
          confirmLabel="Prune"
          onConfirm={async () => {
            const result = await pruneImages();
            toast.push(
              "success",
              result.deleted === 0
                ? "Nothing to prune."
                : `Removed ${result.deleted} image${result.deleted === 1 ? "" : "s"}, reclaimed ${formatBytes(result.spaceReclaimedBytes ?? 0)}.`,
            );
          }}
          onClose={() => setPruneOpen(false)}
        />
      )}

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

function HistoryModal({ image, onClose }: { image: DockerImage; onClose: () => void }) {
  const [layers, setLayers] = useState<ImageLayer[]>();
  const [error, setError] = useState<string>();
  useEffect(() => {
    getImageHistory(image.id)
      .then(setLayers)
      .catch((err: unknown) => setError(err instanceof ApiError ? err.message : "Could not load history."));
  }, [image.id]);
  return (
    <Modal title={`History · ${image.repoTags[0] ?? shortImageId(image.id)}`} size="lg" onClose={onClose}>
      {error && <FormError>{error}</FormError>}
      {!layers && !error && <p className="text-sm text-ink-faint">Loading layers…</p>}
      {layers && (
        <div className="max-h-80 overflow-auto font-mono text-[11px] leading-relaxed">
          {layers.map((layer, i) => (
            <div key={`${layer.id}-${i}`} className="border-b border-edge py-2">
              <div className="flex justify-between gap-3 text-ink-muted">
                <span>{shortImageId(layer.id)}</span>
                <span>{formatBytes(layer.sizeBytes)}</span>
              </div>
              <p className="mt-1 whitespace-pre-wrap break-all text-ink">{layer.createdBy || layer.comment || "—"}</p>
            </div>
          ))}
        </div>
      )}
    </Modal>
  );
}

const SUGGESTIONS = ["nginx:alpine", "redis:7-alpine", "postgres:17", "node:22-alpine"];

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
      await pullImage(reference.trim(), (line) => {
        setLog((prev) => [...prev.slice(-49), line]);
      });
      toast.push("success", `Pulled ${reference.trim()}.`);
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
            Cancel
          </Button>
          <Button variant="primary" onClick={handlePull} loading={pulling} disabled={!reference.trim()}>
            Pull
          </Button>
        </>
      }
    >
      <FieldLabel htmlFor="pull-reference" hint="registry/name:tag">
        Image reference
      </FieldLabel>
      <input
        id="pull-reference"
        type="text"
        autoFocus
        placeholder="nginx:latest"
        value={reference}
        disabled={pulling}
        onChange={(e) => setReference(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && reference.trim() && !pulling) void handlePull();
        }}
        className="field font-mono"
      />
      {!pulling && log.length === 0 && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {SUGGESTIONS.map((s) => (
            <button
              key={s}
              onClick={() => setReference(s)}
              className="rounded border border-edge px-1.5 py-0.5 font-mono text-[11px] text-ink-muted transition hover:border-accent/40 hover:text-accent"
            >
              {s}
            </button>
          ))}
        </div>
      )}

      {log.length > 0 && (
        <div className="mt-3 max-h-44 overflow-y-auto rounded-md border border-edge bg-sunken p-2.5 font-mono text-[11px] leading-relaxed text-ink-muted">
          {log.map((line, i) => (
            <div key={i} className="truncate">
              <span className="text-ink-faint">› </span>
              {line}
            </div>
          ))}
        </div>
      )}

      {error && <FormError>{error}</FormError>}
    </Modal>
  );
}
