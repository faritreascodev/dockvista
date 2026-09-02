import type { Container, ContainerAction, ContainerStats } from "../types/domain";
import { ContainerRow } from "./ContainerRow";
import type { DrawerTab } from "./ContainerDrawer";
import { EmptyState } from "./EmptyState";
import { TableSkeleton } from "./Skeleton";

interface ContainerTableProps {
  containers: Container[] | undefined;
  statsById: Record<string, ContainerStats>;
  loading: boolean;
  selectedId: string | undefined;
  onSelect: (container: Container) => void;
  onOpenTab: (container: Container, tab: DrawerTab) => void;
  onAction: (id: string, action: ContainerAction) => Promise<void>;
}

const COLUMNS = ["Status", "Container", "Image", "CPU", "Memory", "Created", "Actions"];

export function ContainerTable({
  containers,
  statsById,
  loading,
  selectedId,
  onSelect,
  onOpenTab,
  onAction,
}: ContainerTableProps) {
  if (loading && !containers) {
    return (
      <div className="rounded-xl border border-edge bg-panel">
        <TableSkeleton />
      </div>
    );
  }

  if (!containers || containers.length === 0) {
    return (
      <div className="rounded-xl border border-edge bg-panel">
        <EmptyState message="No containers found. Run one with `docker run` to see it here." />
      </div>
    );
  }

  return (
    <div className="overflow-x-auto rounded-xl border border-edge bg-panel">
      <table className="w-full min-w-[720px] border-collapse text-left">
        <thead>
          <tr className="border-b border-edge bg-panel-2 text-xs uppercase tracking-wide text-ink-muted">
            {COLUMNS.map((col) => (
              <th
                key={col}
                className={`py-2.5 font-medium ${col === "Status" ? "pl-4" : ""} ${col === "Actions" ? "pr-4 text-right" : ""}`}
              >
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {containers.map((c) => (
            <ContainerRow
              key={c.id}
              container={c}
              stats={statsById[c.id]}
              selected={c.id === selectedId}
              onSelect={onSelect}
              onOpenTab={onOpenTab}
              onAction={onAction}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}
