import { useMemo, useState } from "react";
import { Eraser, Network, Plus } from "lucide-react";
import { ApiError, createNetwork, pruneNetworks, removeNetwork } from "../api/client";
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
import { useNetworks } from "../hooks/useNetworks";
import { useSearch } from "../hooks/useSearch";
import { useSession } from "../hooks/useSession";
import type { DockerNetwork } from "../types/domain";
import { formatRelativeTime, shortId } from "../utils/format";

const BUILTIN = new Set(["bridge", "host", "none"]);

export function NetworksPage() {
  const { data: networks, error, loading } = useNetworks();
  const { query } = useSearch();
  const { readOnly } = useSession();
  const [createOpen, setCreateOpen] = useState(false);
  const [pruneOpen, setPruneOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<DockerNetwork>();
  const toast = useToast();

  const filtered = useMemo(() => {
    if (!query.trim()) return networks;
    const q = query.trim().toLowerCase();
    return networks?.filter((n) => n.name.toLowerCase().includes(q));
  }, [networks, query]);

  return (
    <main className="flex-1 overflow-y-auto px-4 py-6 md:px-8">
      <PageHeader
        kicker="Resources"
        title="Networks"
        description="Bridge, overlay, and custom networks, with the containers attached to each."
        actions={
          !readOnly && (
            <>
              <Button onClick={() => setPruneOpen(true)}>
                <Eraser className="h-4 w-4" /> Prune unused
              </Button>
              <Button variant="primary" onClick={() => setCreateOpen(true)}>
                <Plus className="h-4 w-4" /> Create network
              </Button>
            </>
          )
        }
      />

      {error && (
        <div className="mt-5">
          <ErrorBanner message={`Failed to load networks: ${error.message}`} />
        </div>
      )}

      <div className="mt-6">
        <TableFrame>
          {loading && !networks ? (
            <TableSkeleton />
          ) : !filtered || filtered.length === 0 ? (
            <EmptyState icon={Network} message={query ? "No networks match your filter." : "No networks found."} />
          ) : (
            <DataTable columns={["Name", "Driver", "Scope", "Attached", "Created", ""]}>
              {filtered.map((n) => {
                const isBuiltin = BUILTIN.has(n.name);
                const attached = Object.values(n.containers ?? {});
                return (
                  <tr key={n.id} className="transition-colors hover:bg-panel-2/60">
                    <td className="py-3 pl-4 pr-4">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-ink">{n.name}</span>
                        {isBuiltin && <Chip>built-in</Chip>}
                        {n.internal && <Chip tone="warn">internal</Chip>}
                      </div>
                      <p className="font-mono text-[11px] text-ink-faint">{shortId(n.id)}</p>
                    </td>
                    <td className="py-3 pr-4">
                      <Chip tone={n.driver === "bridge" ? "muted" : "accent"}>{n.driver}</Chip>
                    </td>
                    <td className="py-3 pr-4 font-mono text-xs text-ink-muted">{n.scope}</td>
                    <td className="max-w-[260px] py-3 pr-4">
                      {attached.length === 0 ? (
                        <span className="font-mono text-xs text-ink-faint">—</span>
                      ) : (
                        <span className="block truncate text-xs text-ink-muted" title={attached.join(", ")}>
                          <span className="font-mono text-ink">{attached.length}</span> · {attached.join(", ")}
                        </span>
                      )}
                    </td>
                    <td className="whitespace-nowrap py-3 pr-4 text-xs text-ink-muted">{formatRelativeTime(n.createdAt)}</td>
                    <td className="py-3 pr-4 text-right">
                      {!readOnly && (
                        <RemoveButton
                          onClick={() => setRemoveTarget(n)}
                          disabled={isBuiltin}
                          title={isBuiltin ? "Built-in networks can't be removed" : "Remove network"}
                        />
                      )}
                    </td>
                  </tr>
                );
              })}
            </DataTable>
          )}
        </TableFrame>
      </div>

      {createOpen && <CreateNetworkModal onClose={() => setCreateOpen(false)} />}

      {pruneOpen && (
        <ConfirmDialog
          title="Prune unused networks"
          description="Removes every custom network with no containers attached. Built-in networks are never touched."
          confirmLabel="Prune"
          onConfirm={async () => {
            const result = await pruneNetworks();
            toast.push(
              "success",
              result.deleted === 0 ? "Nothing to prune." : `Removed ${result.deleted} network${result.deleted === 1 ? "" : "s"}.`,
            );
          }}
          onClose={() => setPruneOpen(false)}
        />
      )}

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
      <FieldLabel htmlFor="network-name">Name</FieldLabel>
      <input
        id="network-name"
        type="text"
        autoFocus
        placeholder="my-network"
        value={name}
        disabled={busy}
        onChange={(e) => setName(e.target.value)}
        className="field font-mono"
      />
      <p className="mt-2 text-xs text-ink-faint">
        Driver: <span className="font-mono">bridge</span>
      </p>
      {error && <FormError>{error}</FormError>}
    </Modal>
  );
}
