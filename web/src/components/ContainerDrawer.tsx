import { useEffect, useRef, useState } from "react";
import { Trash2, X } from "lucide-react";
import { ApiError, inspectContainer } from "../api/client";
import type { Container, ContainerAction, ContainerStats } from "../types/domain";
import { useContainerLogs } from "../hooks/useContainerLogs";
import { formatBytes, formatPercent, formatRelativeTime, shortId } from "../utils/format";
import { StatusBadge } from "./StatusBadge";
import { ContainerActions } from "./ContainerActions";
import { TerminalPanel } from "./TerminalPanel";
import { JsonView } from "./ui/JsonView";

export type DrawerTab = "overview" | "logs" | "stats" | "terminal" | "inspect";

interface ContainerDrawerProps {
  container: Container;
  stats: ContainerStats | undefined;
  tab: DrawerTab;
  onTabChange: (tab: DrawerTab) => void;
  onClose: () => void;
  onAction: (id: string, action: ContainerAction) => Promise<void>;
  onRemove: (container: Container) => void;
}

const TABS: DrawerTab[] = ["overview", "logs", "stats", "terminal", "inspect"];

export function ContainerDrawer({ container, stats, tab, onTabChange, onClose, onAction, onRemove }: ContainerDrawerProps) {
  return (
    <aside className="flex w-[420px] shrink-0 flex-col border-l border-edge bg-panel">
      <div className="flex items-center justify-between border-b border-edge px-4 py-3">
        <div>
          <h2 className="font-semibold text-ink">{container.name}</h2>
          <StatusBadge state={container.state} />
        </div>
        <div className="flex items-center gap-2">
          <ContainerActions container={container} onAction={onAction} size="md" />
          <button
            onClick={() => onRemove(container)}
            title="Remove container"
            className="rounded-md border border-edge bg-panel-2 p-2 text-ink-muted transition hover:bg-rose-100 hover:text-rose-600 dark:hover:bg-rose-950/50 dark:hover:text-rose-400"
          >
            <Trash2 className="h-4 w-4" />
          </button>
          <button
            onClick={onClose}
            className="rounded-md p-1.5 text-ink-muted hover:bg-panel-2 hover:text-ink"
            aria-label="Close"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>

      <div className="flex border-b border-edge">
        {TABS.map((t) => (
          <button
            key={t}
            onClick={() => onTabChange(t)}
            className={`flex-1 border-b-2 py-2 text-sm font-medium capitalize transition-colors ${
              tab === t ? "border-accent text-accent" : "border-transparent text-ink-muted hover:text-ink"
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {tab === "overview" && <OverviewTab container={container} />}
        {tab === "logs" && <LogsTab container={container} />}
        {tab === "stats" && <StatsTab container={container} stats={stats} />}
        {tab === "terminal" && <TerminalPanel containerId={container.id} running={container.state === "running"} />}
        {tab === "inspect" && <InspectTab containerId={container.id} />}
      </div>
    </aside>
  );
}

function InspectTab({ containerId }: { containerId: string }) {
  const [data, setData] = useState<unknown>();
  const [error, setError] = useState<string>();

  useEffect(() => {
    setData(undefined);
    setError(undefined);
    inspectContainer(containerId)
      .then(setData)
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load inspect data."));
  }, [containerId]);

  if (error) return <p className="px-4 py-6 text-sm text-rose-600 dark:text-rose-400">{error}</p>;
  if (data === undefined) return <p className="px-4 py-6 text-sm text-ink-muted">Loading...</p>;
  return <JsonView data={data} />;
}

function OverviewTab({ container }: { container: Container }) {
  const rows: [string, string][] = [
    ["Container ID", container.id],
    ["Image", container.image],
    ["Created", formatRelativeTime(container.createdAt)],
    [
      "Ports",
      container.ports.length
        ? container.ports
            .map((p) => (p.publicPort ? `${p.publicPort}:${p.privatePort}/${p.type}` : `${p.privatePort}/${p.type}`))
            .join(", ")
        : "none",
    ],
    ["Status", container.status],
  ];

  return (
    <dl className="divide-y divide-edge px-4">
      {rows.map(([label, value]) => (
        <div key={label} className="flex items-start justify-between gap-4 py-3">
          <dt className="shrink-0 text-xs text-ink-muted">{label}</dt>
          <dd className="break-all text-right text-xs text-ink">{value}</dd>
        </div>
      ))}
      {Object.keys(container.labels ?? {}).length > 0 && (
        <div className="py-3">
          <dt className="mb-2 text-xs text-ink-muted">Labels</dt>
          <dd className="space-y-1">
            {Object.entries(container.labels ?? {}).map(([k, v]) => (
              <div key={k} className="break-all rounded bg-panel-2 px-2 py-1 font-mono text-[11px] text-ink-muted">
                {k}={v}
              </div>
            ))}
          </dd>
        </div>
      )}
    </dl>
  );
}

function LogsTab({ container }: { container: Container }) {
  const { lines, state } = useContainerLogs(container.id);
  const [autoScroll, setAutoScroll] = useState(true);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (autoScroll) bottomRef.current?.scrollIntoView({ block: "end" });
  }, [lines, autoScroll]);

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-edge px-4 py-2 text-xs text-ink-muted">
        <span>{state === "open" ? "Streaming live" : state === "connecting" ? "Connecting..." : "Disconnected"}</span>
        <label className="flex items-center gap-1.5">
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
            className="accent-accent"
          />
          Auto-scroll
        </label>
      </div>
      <div className="flex-1 overflow-y-auto bg-panel-2 px-4 py-2 font-mono text-xs leading-relaxed">
        {lines.length === 0 && <p className="text-ink-faint">Waiting for log output...</p>}
        {lines.map((line, i) => (
          <p key={i} className={line.stream === "stderr" ? "text-rose-500 dark:text-rose-400" : "text-ink"}>
            {line.message}
          </p>
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}

function StatsTab({ container, stats }: { container: Container; stats: ContainerStats | undefined }) {
  if (container.state !== "running") {
    return <p className="px-4 py-6 text-sm text-ink-muted">Stats are only available for running containers.</p>;
  }

  if (!stats) {
    return <p className="px-4 py-6 text-sm text-ink-muted">Loading stats...</p>;
  }

  return (
    <div className="space-y-4 px-4 py-4">
      <div>
        <div className="flex items-center justify-between text-xs text-ink-muted">
          <span>CPU Usage</span>
          <span className="tabular-nums text-ink">{formatPercent(stats.cpuPercent)}</span>
        </div>
        <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-panel-2">
          <div className="h-full rounded-full bg-accent" style={{ width: `${Math.min(100, stats.cpuPercent)}%` }} />
        </div>
      </div>

      <div>
        <div className="flex items-center justify-between text-xs text-ink-muted">
          <span>Memory Usage</span>
          <span className="tabular-nums text-ink">
            {formatBytes(stats.memoryUsageBytes)} / {formatBytes(stats.memoryLimitBytes)}
          </span>
        </div>
        <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-panel-2">
          <div
            className="h-full rounded-full bg-emerald-500"
            style={{ width: `${Math.min(100, stats.memoryPercent)}%` }}
          />
        </div>
      </div>

      <p className="pt-2 text-xs text-ink-faint">Container: {shortId(container.id)}</p>
    </div>
  );
}
