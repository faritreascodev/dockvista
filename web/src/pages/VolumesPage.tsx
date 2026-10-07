import { useEffect, useMemo, useState } from "react";
import { Eraser, HardDrive, Plus } from "lucide-react";
import { ApiError, createVolume, listVolumeFiles, pruneVolumes, removeVolume, volumeFileContentUrl } from "../api/client";
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
import { useSearch } from "../hooks/useSearch";
import { useSession } from "../hooks/useSession";
import { useVolumes } from "../hooks/useVolumes";
import type { DirListing, DockerVolume } from "../types/domain";
import { formatBytes, formatRelativeTime } from "../utils/format";

const PROJECT_LABEL = "com.docker.compose.project";

export function VolumesPage() {
  const { data: volumes, error, loading } = useVolumes();
  const { query } = useSearch();
  const { readOnly } = useSession();
  const [createOpen, setCreateOpen] = useState(false);
  const [pruneOpen, setPruneOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<DockerVolume>();
  const [browseTarget, setBrowseTarget] = useState<DockerVolume>();
  const toast = useToast();

  const filtered = useMemo(() => {
    if (!query.trim()) return volumes;
    const q = query.trim().toLowerCase();
    return volumes?.filter((v) => v.name.toLowerCase().includes(q));
  }, [volumes, query]);

  return (
    <main className="flex-1 overflow-y-auto px-4 py-6 md:px-8">
      <PageHeader
        kicker="Resources"
        title="Volumes"
        description="Persistent storage managed by the Docker daemon."
        actions={
          !readOnly && (
            <>
              <Button onClick={() => setPruneOpen(true)}>
                <Eraser className="h-4 w-4" /> Prune unused
              </Button>
              <Button variant="primary" onClick={() => setCreateOpen(true)}>
                <Plus className="h-4 w-4" /> Create volume
              </Button>
            </>
          )
        }
      />

      {error && (
        <div className="mt-5">
          <ErrorBanner message={`Failed to load volumes: ${error.message}`} />
        </div>
      )}

      <div className="mt-6">
        <TableFrame>
          {loading && !volumes ? (
            <TableSkeleton />
          ) : !filtered || filtered.length === 0 ? (
            <EmptyState
              icon={HardDrive}
              message={query ? "No volumes match your filter." : "No volumes yet. Create one to persist container data."}
            />
          ) : (
            <DataTable columns={["Name", "Driver", "Mountpoint", "Created", ""]}>
              {filtered.map((v) => {
                const project = v.labels?.[PROJECT_LABEL];
                return (
                  <tr key={v.name} className="transition-colors hover:bg-panel-2/60">
                    <td className="max-w-[280px] py-3 pl-4 pr-4">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-mono text-xs text-ink" title={v.name}>
                          {v.name}
                        </span>
                        {project && <Chip tone="accent">{project}</Chip>}
                      </div>
                    </td>
                    <td className="py-3 pr-4">
                      <Chip>{v.driver}</Chip>
                    </td>
                    <td className="max-w-xs truncate py-3 pr-4 font-mono text-[11px] text-ink-faint" title={v.mountpoint}>
                      {v.mountpoint}
                    </td>
                    <td className="whitespace-nowrap py-3 pr-4 text-xs text-ink-muted">{formatRelativeTime(v.createdAt)}</td>
                    <td className="py-3 pr-4 text-right">
                      <div className="flex justify-end gap-2">
                        <button type="button" className="text-xs text-accent hover:underline" onClick={() => setBrowseTarget(v)}>
                          Browse
                        </button>
                        {!readOnly && <RemoveButton title="Remove volume" onClick={() => setRemoveTarget(v)} />}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </DataTable>
          )}
        </TableFrame>
      </div>

      {browseTarget && <VolumeBrowseModal volume={browseTarget} onClose={() => setBrowseTarget(undefined)} />}

      {createOpen && <CreateVolumeModal onClose={() => setCreateOpen(false)} />}

      {pruneOpen && (
        <ConfirmDialog
          title="Prune unused volumes"
          description="Deletes every anonymous volume not attached to a container, along with the data inside. This cannot be undone."
          confirmLabel="Prune"
          onConfirm={async () => {
            const result = await pruneVolumes();
            toast.push(
              "success",
              result.deleted === 0
                ? "Nothing to prune."
                : `Removed ${result.deleted} volume${result.deleted === 1 ? "" : "s"}, reclaimed ${formatBytes(result.spaceReclaimedBytes ?? 0)}.`,
            );
          }}
          onClose={() => setPruneOpen(false)}
        />
      )}

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

function VolumeBrowseModal({ volume, onClose }: { volume: DockerVolume; onClose: () => void }) {
  const [path, setPath] = useState("/");
  const [listing, setListing] = useState<DirListing>();
  const [error, setError] = useState<string>();

  useEffect(() => {
    setListing(undefined);
    setError(undefined);
    listVolumeFiles(volume.name, path)
      .then(setListing)
      .catch((err: unknown) => setError(err instanceof ApiError ? err.message : "Could not list volume."));
  }, [volume.name, path]);

  return (
    <Modal title={`Browse · ${volume.name}`} size="lg" onClose={onClose}>
      <p className="mb-3 text-xs text-ink-faint">
        Uses a running container that mounts this volume. If none is running, start one — we do not spawn an inspector container.
      </p>
      {error && <FormError>{error}</FormError>}
      {listing?.reason && (
        <p className="text-sm text-ink-muted">
          {listing.reason === "not_running"
            ? "No running container currently mounts this volume."
            : listing.reason}
        </p>
      )}
      {listing && !listing.reason && (
        <ul className="max-h-72 overflow-auto font-mono text-xs">
          {path !== "/" && (
            <li>
              <button type="button" className="text-accent" onClick={() => setPath(path.replace(/\/[^/]+$/, "") || "/")}>
                ../
              </button>
            </li>
          )}
          {listing.entries.map((e) => (
            <li key={e.path} className="flex items-center justify-between py-1">
              {e.dir ? (
                <button type="button" className="text-left text-accent" onClick={() => setPath(e.path)}>
                  {e.name}/
                </button>
              ) : (
                <a className="text-ink hover:underline" href={volumeFileContentUrl(volume.name, e.path)}>
                  {e.name}
                </a>
              )}
              <span className="text-ink-faint">{e.dir ? "dir" : formatBytes(e.sizeBytes)}</span>
            </li>
          ))}
        </ul>
      )}
    </Modal>
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
      <FieldLabel htmlFor="volume-name" hint="optional, auto-generated if blank">
        Name
      </FieldLabel>
      <input
        id="volume-name"
        type="text"
        autoFocus
        placeholder="my-data"
        value={name}
        disabled={busy}
        onChange={(e) => setName(e.target.value)}
        className="field font-mono"
      />
      <p className="mt-2 text-xs text-ink-faint">
        Driver: <span className="font-mono">local</span>
      </p>
      {error && <FormError>{error}</FormError>}
    </Modal>
  );
}
