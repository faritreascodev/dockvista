import { useMemo } from "react";
import { Hexagon } from "lucide-react";
import { getSwarm } from "../api/client";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/Skeleton";
import { Chip, DataTable, TableFrame } from "../components/ui/Table";
import { usePolling } from "../hooks/usePolling";
import { useSearch } from "../hooks/useSearch";
import { shortId } from "../utils/format";

const POLL_INTERVAL_MS = 15000;

export function SwarmPage() {
  const { data, error, loading } = usePolling(getSwarm, POLL_INTERVAL_MS, []);
  const { query } = useSearch();
  const q = query.trim().toLowerCase();

  const nodes = useMemo(() => {
    const list = data?.nodes ?? [];
    if (!q) return list;
    return list.filter((n) => `${n.hostname} ${n.role} ${n.status} ${n.id}`.toLowerCase().includes(q));
  }, [data, q]);

  const services = useMemo(() => {
    const list = data?.services ?? [];
    if (!q) return list;
    return list.filter((s) => `${s.name} ${s.image} ${s.mode}`.toLowerCase().includes(q));
  }, [data, q]);

  const inactive = data?.state === "inactive" || data?.state === "";

  return (
    <main className="flex-1 overflow-y-auto px-4 py-6 md:px-8">
      <PageHeader
        kicker="Workloads"
        title="Swarm"
        description="Nodes and services on the active engine. Init, join, and scale stay off until this view is honest on a cluster that is actually Swarm."
      />

      {error && (
        <div className="mt-5">
          <ErrorBanner message={`Failed to load swarm: ${error.message}`} />
        </div>
      )}

      {loading && !data ? (
        <div className="mt-6">
          <TableSkeleton />
        </div>
      ) : inactive ? (
        <div className="mt-6 rounded-lg border border-edge bg-panel">
          <EmptyState
            icon={Hexagon}
            message="This engine is not in Swarm mode. Overlay networks and services show up here after an init or join — not on a typical Docker Desktop node."
          />
        </div>
      ) : (
        <div className="mt-6 space-y-8">
          <section>
            <div className="mb-3 flex items-baseline justify-between gap-3">
              <h2 className="text-sm font-semibold text-ink">Nodes</h2>
              <p className="text-[11px] text-ink-faint">
                {data?.controlAvailable ? "manager" : "worker"} · {data?.state}
                {data?.clusterId ? ` · ${shortId(data.clusterId)}` : ""}
              </p>
            </div>
            {data?.error && <p className="mb-3 text-sm text-bad">{data.error}</p>}
            <TableFrame>
              {nodes.length === 0 ? (
                <EmptyState icon={Hexagon} message={q ? "No nodes match your filter." : "No nodes listed (this node may not be a manager)."} />
              ) : (
                <DataTable columns={["Hostname", "Role", "Status", "Availability", "Addr", "Engine"]}>
                  {nodes.map((n) => (
                    <tr key={n.id} className="transition-colors hover:bg-panel-2/60">
                      <td className="py-3 pl-4 pr-4">
                        <p className="font-medium text-ink">{n.hostname || shortId(n.id)}</p>
                        <p className="font-mono text-[11px] text-ink-faint">{shortId(n.id)}</p>
                      </td>
                      <td className="pr-4">
                        <Chip tone={n.leader ? "accent" : "muted"}>{n.leader ? "leader" : n.role || (n.manager ? "manager" : "worker")}</Chip>
                      </td>
                      <td className="pr-4">
                        <Chip tone={n.status === "ready" ? "ok" : "warn"}>{n.status || "unknown"}</Chip>
                      </td>
                      <td className="pr-4 text-ink-muted">{n.availability}</td>
                      <td className="pr-4 font-mono text-xs text-ink-muted">{n.addr}</td>
                      <td className="pr-4 font-mono text-xs text-ink-muted">{n.engineVersion}</td>
                    </tr>
                  ))}
                </DataTable>
              )}
            </TableFrame>
          </section>

          <section>
            <h2 className="mb-3 text-sm font-semibold text-ink">Services</h2>
            <TableFrame>
              {services.length === 0 ? (
                <EmptyState icon={Hexagon} message={q ? "No services match your filter." : "No swarm services."} />
              ) : (
                <DataTable columns={["Name", "Image", "Mode", "Replicas", "Published"]}>
                  {services.map((s) => (
                    <tr key={s.id} className="transition-colors hover:bg-panel-2/60">
                      <td className="py-3 pl-4 pr-4">
                        <p className="font-medium text-ink">{s.name}</p>
                        <p className="font-mono text-[11px] text-ink-faint">{shortId(s.id)}</p>
                      </td>
                      <td className="pr-4 font-mono text-xs text-ink-muted">{s.image}</td>
                      <td className="pr-4">
                        <Chip>{s.mode}</Chip>
                      </td>
                      <td className="pr-4 font-mono text-sm tabular-nums text-ink">
                        {s.running}/{s.replicas || "—"}
                      </td>
                      <td className="pr-4 font-mono text-xs text-ink-muted">{(s.ports ?? []).join(", ") || "—"}</td>
                    </tr>
                  ))}
                </DataTable>
              )}
            </TableFrame>
          </section>
        </div>
      )}
    </main>
  );
}
