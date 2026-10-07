import { Boxes } from "lucide-react";
import type { Container, ContainerAction, ContainerStats } from "../types/domain";
import { ContainerRow } from "./ContainerRow";
import type { DrawerTab } from "./ContainerDrawer";
import { EmptyState } from "./EmptyState";
import { TableSkeleton } from "./Skeleton";

interface ContainerTableProps {
  containers: Container[] | undefined;
  statsById: Record<string, ContainerStats>;
  cpuSeriesById: Record<string, number[]>;
  loading: boolean;
  selectedId: string | undefined;
  onSelect: (container: Container) => void;
  onOpenTab: (container: Container, tab: DrawerTab) => void;
  onAction: (id: string, action: ContainerAction) => Promise<void>;
}

const COLUMNS: { label: string; className?: string }[] = [
  { label: "State", className: "pl-4 w-[104px]" },
  { label: "Container" },
  { label: "Image", className: "hidden lg:table-cell" },
  { label: "CPU", className: "w-[132px]" },
  { label: "Memory", className: "w-[112px]" },
  { label: "Created", className: "hidden xl:table-cell" },
  { label: "" },
];

export function ContainerTable({
  containers,
  statsById,
  cpuSeriesById,
  loading,
  selectedId,
  onSelect,
  onOpenTab,
  onAction,
}: ContainerTableProps) {
  if (loading && !containers) {
    return (
      <div className="rounded-lg border border-edge bg-panel">
        <TableSkeleton />
      </div>
    );
  }

  if (!containers || containers.length === 0) {
    return (
      <div className="rounded-lg border border-edge bg-panel">
        <EmptyState icon={Boxes} message="No containers match. Run one with `docker run`, or create one here." />
      </div>
    );
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-edge bg-panel">
      <table className="w-full min-w-[640px] table-fixed border-collapse text-left">
        <thead>
          <tr className="border-b border-edge bg-panel-2/60">
            {COLUMNS.map((col, i) => (
              <th key={i} className={`label-caps py-2.5 pr-4 font-medium ${col.className ?? ""} ${i === COLUMNS.length - 1 ? "w-[172px]" : ""}`}>
                {col.label}
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
              cpuSeries={cpuSeriesById[c.id]}
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
