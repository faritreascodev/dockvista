import { FolderOpen, ScrollText, SquareTerminal } from "lucide-react";
import { useSession } from "../hooks/useSession";
import type { Container, ContainerAction, ContainerStats } from "../types/domain";
import { formatBytes, formatPercent, formatRelativeTime, shortId } from "../utils/format";
import { ContainerActions } from "./ContainerActions";
import type { DrawerTab } from "./ContainerDrawer";
import { Sparkline } from "./Sparkline";
import { StatusBadge } from "./StatusBadge";

interface ContainerRowProps {
  container: Container;
  stats: ContainerStats | undefined;
  cpuSeries: number[] | undefined;
  selected: boolean;
  onSelect: (container: Container) => void;
  onOpenTab: (container: Container, tab: DrawerTab) => void;
  onAction: (id: string, action: ContainerAction) => Promise<void>;
}

function formatPorts(container: Container): string {
  const published = container.ports.filter((p) => p.publicPort);
  const first = published[0];
  if (!first) return "";
  const rest = published.length - 1;
  return `:${first.publicPort}→${first.privatePort}${rest > 0 ? ` +${rest}` : ""}`;
}

const ICON_BUTTON =
  "flex h-7 w-7 items-center justify-center rounded-md border border-edge bg-panel text-ink-muted transition hover:border-accent/40 hover:text-accent disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:border-edge disabled:hover:text-ink-muted";

export function ContainerRow({ container, stats, cpuSeries, selected, onSelect, onOpenTab, onAction }: ContainerRowProps) {
  const isRunning = container.state === "running";
  const { readOnly } = useSession();
  const ports = formatPorts(container);

  return (
    <tr
      onClick={() => onSelect(container)}
      className={`group cursor-pointer border-b border-edge text-sm transition-colors last:border-b-0 hover:bg-panel-2 ${
        selected ? "bg-accent/[0.06] shadow-[inset_3px_0_0_rgb(var(--color-accent))]" : ""
      }`}
    >
      <td className="py-3 pl-4">
        <StatusBadge state={container.state} />
      </td>
      <td className="py-3 pr-4">
        <p className="truncate font-medium text-ink" title={container.name}>
          {container.name}
        </p>
        <p className="truncate font-mono text-[11px] text-ink-faint">
          {shortId(container.id)}
          {ports && <span className="ml-2 text-ink-muted">{ports}</span>}
        </p>
      </td>
      <td className="hidden truncate py-3 pr-4 font-mono text-xs text-ink-muted lg:table-cell" title={container.image}>
        {container.image}
      </td>
      <td className="py-3 pr-4">
        {isRunning && stats ? (
          <div className="flex items-center gap-2.5">
            <Sparkline values={cpuSeries ?? []} max={Math.max(5, ...(cpuSeries ?? []))} className="h-6 w-14" />
            <span className="w-12 font-mono text-xs tabular-nums text-ink">{formatPercent(stats.cpuPercent)}</span>
          </div>
        ) : (
          <span className="font-mono text-xs text-ink-faint">—</span>
        )}
      </td>
      <td className="py-3 pr-4">
        {isRunning && stats ? (
          <div>
            <span className="font-mono text-xs tabular-nums text-ink">{formatBytes(stats.memoryUsageBytes)}</span>
            <div className="mt-1 h-[3px] w-full max-w-20 overflow-hidden rounded-full bg-edge">
              <div
                className={`h-full rounded-full ${stats.memoryPercent > 85 ? "bg-bad" : stats.memoryPercent > 60 ? "bg-warn" : "bg-info"}`}
                style={{ width: `${Math.min(100, stats.memoryPercent)}%` }}
              />
            </div>
          </div>
        ) : (
          <span className="font-mono text-xs text-ink-faint">—</span>
        )}
      </td>
      <td className="hidden whitespace-nowrap py-3 pr-4 text-xs text-ink-muted xl:table-cell">
        {formatRelativeTime(container.createdAt)}
      </td>
      <td className="py-3 pr-4">
        <div className="flex items-center justify-end gap-1">
          <button
            title="View logs"
            aria-label={`View logs for ${container.name}`}
            onClick={(e) => {
              e.stopPropagation();
              onOpenTab(container, "logs");
            }}
            className={ICON_BUTTON}
          >
            <ScrollText className="h-3.5 w-3.5" />
          </button>
          <button
            title="Browse files"
            aria-label={`Browse files in ${container.name}`}
            onClick={(e) => {
              e.stopPropagation();
              onOpenTab(container, "files");
            }}
            className={ICON_BUTTON}
          >
            <FolderOpen className="h-3.5 w-3.5" />
          </button>
          {!readOnly && (
            <button
              title={isRunning ? "Open terminal" : "Container must be running"}
              aria-label={`Open terminal in ${container.name}`}
              disabled={!isRunning}
              onClick={(e) => {
                e.stopPropagation();
                onOpenTab(container, "terminal");
              }}
              className={ICON_BUTTON}
            >
              <SquareTerminal className="h-3.5 w-3.5" />
            </button>
          )}
          <ContainerActions container={container} onAction={onAction} />
        </div>
      </td>
    </tr>
  );
}
