import { ScrollText, SquareTerminal } from "lucide-react";
import type { Container, ContainerAction, ContainerStats } from "../types/domain";
import { formatBytes, formatPercent, formatRelativeTime, shortId } from "../utils/format";
import { StatusBadge } from "./StatusBadge";
import { ContainerActions } from "./ContainerActions";
import type { DrawerTab } from "./ContainerDrawer";

interface ContainerRowProps {
  container: Container;
  stats: ContainerStats | undefined;
  selected: boolean;
  onSelect: (container: Container) => void;
  onOpenTab: (container: Container, tab: DrawerTab) => void;
  onAction: (id: string, action: ContainerAction) => Promise<void>;
}

function MiniBar({ percent, colorClass }: { percent: number; colorClass: string }) {
  return (
    <div className="mt-1 h-1 w-16 overflow-hidden rounded-full bg-panel-2">
      <div className={`h-full rounded-full ${colorClass}`} style={{ width: `${Math.min(100, percent)}%` }} />
    </div>
  );
}

export function ContainerRow({ container, stats, selected, onSelect, onOpenTab, onAction }: ContainerRowProps) {
  const isRunning = container.state === "running";

  return (
    <tr
      onClick={() => onSelect(container)}
      className={`cursor-pointer border-b border-edge/60 text-sm transition-colors hover:bg-panel-2 ${
        selected ? "bg-accent/5" : ""
      }`}
    >
      <td className="py-2.5 pl-4">
        <StatusBadge state={container.state} />
      </td>
      <td className="py-2.5">
        <p className="font-medium text-ink">{container.name}</p>
        <p className="font-mono text-xs text-ink-faint">{shortId(container.id)}</p>
      </td>
      <td className="max-w-[180px] truncate py-2.5 text-xs text-ink-muted">{container.image}</td>
      <td className="py-2.5 text-xs tabular-nums text-ink-muted">
        {isRunning && stats ? (
          <>
            {formatPercent(stats.cpuPercent)}
            <MiniBar percent={stats.cpuPercent} colorClass="bg-sky-500" />
          </>
        ) : (
          "--"
        )}
      </td>
      <td className="py-2.5 text-xs tabular-nums text-ink-muted">
        {isRunning && stats ? (
          <>
            {formatBytes(stats.memoryUsageBytes)}
            <MiniBar percent={stats.memoryPercent} colorClass="bg-emerald-500" />
          </>
        ) : (
          "--"
        )}
      </td>
      <td className="py-2.5 text-xs text-ink-muted">{formatRelativeTime(container.createdAt)}</td>
      <td className="py-2.5 pr-4 text-right">
        <div className="flex items-center justify-end gap-1">
          <button
            title="View logs"
            onClick={(e) => {
              e.stopPropagation();
              onOpenTab(container, "logs");
            }}
            className="rounded-md border border-edge bg-panel-2 p-1.5 text-ink-muted transition hover:bg-accent/10 hover:text-accent"
          >
            <ScrollText className="h-3.5 w-3.5" />
          </button>
          <button
            title={isRunning ? "Open terminal" : "Container must be running"}
            disabled={!isRunning}
            onClick={(e) => {
              e.stopPropagation();
              onOpenTab(container, "terminal");
            }}
            className="rounded-md border border-edge bg-panel-2 p-1.5 text-ink-muted transition hover:bg-accent/10 hover:text-accent disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-panel-2 disabled:hover:text-ink-muted"
          >
            <SquareTerminal className="h-3.5 w-3.5" />
          </button>
          <ContainerActions container={container} onAction={onAction} />
        </div>
      </td>
    </tr>
  );
}
