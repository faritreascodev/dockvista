import { lazy, Suspense, useEffect, useRef, useState, type ReactNode } from "react";
import { Download, Trash2, X } from "lucide-react";
import { ApiError, downloadBlob, getContainerStats, inspectContainer, logsDownloadUrl } from "../api/client";
import type { Container, ContainerAction, ContainerStats } from "../types/domain";
import { useContainerLogs } from "../hooks/useContainerLogs";
import { useFleetStats } from "../hooks/useFleetStats";
import { usePageVisible } from "../hooks/usePageVisible";
import { usePolling } from "../hooks/usePolling";
import { useSession } from "../hooks/useSession";
import { formatBytes, formatBytesPerSec, formatPercent, formatRelativeTime } from "../utils/format";
import { ContainerActions } from "./ContainerActions";
import { FilesTab } from "./FilesTab";
import { MountsTab } from "./MountsTab";
import { Sparkline } from "./Sparkline";
import { StatusBadge } from "./StatusBadge";
import { JsonView } from "./ui/JsonView";
import { useToast } from "./ui/toastContext";

// xterm is most of the bundle; only fetch it once a shell is opened.
const TerminalPanel = lazy(() => import("./TerminalPanel").then((m) => ({ default: m.TerminalPanel })));

export type DrawerTab = "overview" | "logs" | "stats" | "files" | "mounts" | "terminal" | "inspect";

interface ContainerDrawerProps {
  container: Container;
  stats: ContainerStats | undefined;
  tab: DrawerTab;
  onTabChange: (tab: DrawerTab) => void;
  onClose: () => void;
  onAction: (id: string, action: ContainerAction) => Promise<void>;
  onRemove: (container: Container) => void;
}

const ALL_TABS: DrawerTab[] = ["overview", "logs", "stats", "files", "mounts", "terminal", "inspect"];

export function ContainerDrawer({ container, stats, tab, onTabChange, onClose, onAction, onRemove }: ContainerDrawerProps) {
  const { readOnly } = useSession();
  const tabs = readOnly ? ALL_TABS.filter((t) => t !== "terminal") : ALL_TABS;
  const activeTab = tabs.includes(tab) ? tab : "overview";
  const [filesPath, setFilesPath] = useState("/");

  useEffect(() => {
    setFilesPath("/");
  }, [container.id]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      // Escape inside the terminal belongs to the shell, and a modal or the
      // palette that handled it already marks it defaultPrevented.
      if (e.key !== "Escape" || e.defaultPrevented) return;
      if (e.target instanceof HTMLElement && (e.target.closest(".xterm") || e.target.closest("[role=dialog]"))) return;
      onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <aside className="fixed inset-y-0 right-0 z-30 flex w-full max-w-[540px] shrink-0 flex-col border-l border-edge bg-panel shadow-2xl animate-slide-in xl:static xl:z-auto xl:shadow-none">
      <div className="border-b border-edge px-4 pb-3 pt-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <StatusBadge state={container.state} />
            <h2 className="mt-1 truncate text-lg font-semibold tracking-tight text-ink">{container.name}</h2>
            <p className="truncate font-mono text-[11px] text-ink-faint">{container.image}</p>
          </div>
          <button
            onClick={onClose}
            className="rounded-md p-1.5 text-ink-muted hover:bg-panel-2 hover:text-ink"
            aria-label="Close details"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        {!readOnly && (
          <div className="mt-3 flex items-center gap-1.5">
            <ContainerActions container={container} onAction={onAction} size="md" />
            <button
              onClick={() => onRemove(container)}
              title="Remove container"
              aria-label={`Remove ${container.name}`}
              className="ml-auto flex h-8 items-center gap-1.5 rounded-md border border-edge px-2.5 text-xs text-ink-muted transition hover:border-bad/40 hover:text-bad"
            >
              <Trash2 className="h-3.5 w-3.5" /> Remove
            </button>
          </div>
        )}
      </div>

      <div className="flex gap-1 overflow-x-auto border-b border-edge px-3" role="tablist">
        {tabs.map((t) => (
          <button
            key={t}
            role="tab"
            aria-selected={activeTab === t}
            onClick={() => onTabChange(t)}
            className={`-mb-px border-b-2 px-2.5 py-2.5 font-mono text-[11px] font-medium uppercase tracking-wider transition-colors ${
              activeTab === t ? "border-accent text-ink" : "border-transparent text-ink-faint hover:text-ink-muted"
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      <div
        className={`min-h-0 flex-1 ${
          activeTab === "logs" || activeTab === "files" || activeTab === "terminal" ? "overflow-hidden" : "overflow-y-auto"
        }`}
      >
        {activeTab === "overview" && <OverviewTab container={container} />}
        {activeTab === "logs" && <LogsTab container={container} />}
        {activeTab === "stats" && <StatsTab container={container} stats={stats} />}
        {activeTab === "files" && <FilesTab container={container} jumpPath={filesPath} />}
        {activeTab === "mounts" && (
          <MountsTab
            container={container}
            onOpenPath={(p) => {
              setFilesPath(p);
              onTabChange("files");
            }}
          />
        )}
        {activeTab === "terminal" && (
          <Suspense fallback={<p className="px-4 py-6 font-mono text-xs text-ink-faint">Loading terminal…</p>}>
            <TerminalPanel containerId={container.id} running={container.state === "running"} />
          </Suspense>
        )}
        {activeTab === "inspect" && <InspectTab containerId={container.id} />}
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

  if (error) return <p className="px-4 py-6 text-sm text-bad">{error}</p>;
  if (data === undefined) return <p className="px-4 py-6 text-sm text-ink-muted">Loading…</p>;
  return <JsonView data={data} />;
}

function OverviewTab({ container }: { container: Container }) {
  const rows: [string, string][] = [
    ["ID", container.id],
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
  const labels = Object.entries(container.labels ?? {});

  return (
    <div className="px-4 py-2">
      <dl className="divide-y divide-edge">
        {rows.map(([label, value]) => (
          <div key={label} className="grid grid-cols-[88px_1fr] gap-3 py-2.5">
            <dt className="label-caps pt-0.5">{label}</dt>
            <dd className="break-all font-mono text-xs text-ink">{value}</dd>
          </div>
        ))}
      </dl>
      {labels.length > 0 && (
        <div className="border-t border-edge py-3">
          <p className="label-caps mb-2">Labels · {labels.length}</p>
          <div className="space-y-1">
            {labels.map(([k, v]) => (
              <div key={k} className="break-all rounded border border-edge bg-panel-2 px-2 py-1 font-mono text-[11px]">
                <span className="text-ink-muted">{k}</span>
                <span className="text-ink-faint">=</span>
                <span className="text-ink">{v}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function LogsTab({ container }: { container: Container }) {
  const [autoScroll, setAutoScroll] = useState(true);
  const [filter, setFilter] = useState("");
  const [timestamps, setTimestamps] = useState(false);
  const [since, setSince] = useState("");
  const [downloading, setDownloading] = useState(false);
  const logQuery = { timestamps, ...(since ? { since } : {}) };
  const { lines, state } = useContainerLogs(container.id, logQuery);
  const bottomRef = useRef<HTMLDivElement>(null);
  const toast = useToast();

  const needle = filter.trim().toLowerCase();
  const visible = needle
    ? lines.filter((l) => l.message.toLowerCase().includes(needle) || (l.timestamp ?? "").toLowerCase().includes(needle))
    : lines;

  useEffect(() => {
    if (autoScroll) bottomRef.current?.scrollIntoView({ block: "end" });
  }, [visible.length, autoScroll]);

  async function download() {
    setDownloading(true);
    try {
      await downloadBlob(logsDownloadUrl(container.id, { timestamps, tail: "all", ...(since ? { since } : {}) }), `${container.name}.log`);
    } catch (err) {
      toast.push("error", err instanceof ApiError ? err.message : "Log download failed.");
    } finally {
      setDownloading(false);
    }
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex flex-wrap items-center gap-2 border-b border-edge px-4 py-2">
        <span className="flex items-center gap-1.5 font-mono text-[11px] text-ink-muted">
          <span
            className={`h-1.5 w-1.5 rounded-full ${state === "open" ? "bg-ok live-dot" : state === "connecting" ? "bg-warn" : "bg-ink-faint"}`}
          />
          {state === "open" ? "live" : state === "connecting" ? "connecting" : "disconnected"}
        </span>
        <input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="grep…"
          className="h-7 min-w-0 flex-1 rounded border border-edge bg-panel-2 px-2 font-mono text-xs text-ink outline-none placeholder:text-ink-faint focus:border-accent"
        />
        <select
          value={since}
          onChange={(e) => setSince(e.target.value)}
          className="h-7 rounded border border-edge bg-panel-2 px-1.5 font-mono text-[11px] text-ink"
          aria-label="Since"
        >
          <option value="">all</option>
          <option value="5m">5m</option>
          <option value="1h">1h</option>
        </select>
        <label className="flex shrink-0 items-center gap-1.5 text-xs text-ink-muted">
          <input type="checkbox" checked={timestamps} onChange={(e) => setTimestamps(e.target.checked)} className="accent-accent" />
          ts
        </label>
        <label className="flex shrink-0 items-center gap-1.5 text-xs text-ink-muted">
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
            className="accent-accent"
          />
          Follow
        </label>
        <button
          type="button"
          onClick={download}
          disabled={downloading}
          className="flex h-7 items-center gap-1 rounded border border-edge px-2 text-[11px] text-ink-muted hover:text-ink disabled:opacity-50"
        >
          <Download className="h-3 w-3" />
          Save
        </button>
      </div>
      <div className="flex-1 overflow-y-auto bg-sunken px-4 py-2 font-mono text-xs leading-relaxed">
        {visible.length === 0 && (
          <p className="text-ink-faint">{lines.length === 0 ? "Waiting for log output…" : "No lines match the filter."}</p>
        )}
        {visible.map((line, i) => (
          <p
            key={i}
            className={`whitespace-pre-wrap break-all border-l-2 pl-2 text-ink ${line.stream === "stderr" ? "border-bad/50" : "border-transparent"}`}
          >
            {line.timestamp && <span className="mr-2 text-ink-faint">{line.timestamp.replace(/T/, " ").replace(/Z$/, "")}</span>}
            {line.message}
          </p>
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}

function StatsTab({ container, stats }: { container: Container; stats: ContainerStats | undefined }) {
  const { cpuSeriesById } = useFleetStats();
  const visible = usePageVisible();
  const live = usePolling(
    () => getContainerStats(container.id),
    container.state === "running" && visible ? 1000 : null,
    [container.id, container.state],
  );
  const sample = live.data ?? stats;
  const [hist, setHist] = useState({ cpu: [] as number[], mem: [] as number[], rx: [] as number[], tx: [] as number[] });
  const prev = useRef<ContainerStats>();

  useEffect(() => {
    if (!live.data) return;
    const cur = live.data;
    const last = prev.current;
    const dt = last && cur.sampledAt > last.sampledAt ? (cur.sampledAt - last.sampledAt) / 1000 : 1;
    const rx = last ? (cur.netRxBytes - last.netRxBytes) / dt : 0;
    const tx = last ? (cur.netTxBytes - last.netTxBytes) / dt : 0;
    prev.current = cur;
    setHist((h) => ({
      cpu: [...h.cpu.slice(-59), cur.cpuPercent],
      mem: [...h.mem.slice(-59), cur.memoryPercent],
      rx: [...h.rx.slice(-59), Math.max(0, rx)],
      tx: [...h.tx.slice(-59), Math.max(0, tx)],
    }));
  }, [live.data]);

  useEffect(() => {
    prev.current = undefined;
    setHist({ cpu: [], mem: [], rx: [], tx: [] });
  }, [container.id]);

  if (container.state !== "running") {
    return <p className="px-4 py-6 text-sm text-ink-muted">Stats are only available for running containers.</p>;
  }
  if (!sample) {
    return <p className="px-4 py-6 text-sm text-ink-muted">Sampling…</p>;
  }

  const series = hist.cpu.length > 1 ? hist.cpu : (cpuSeriesById[container.id] ?? []);
  const peak = series.length ? Math.max(...series) : 0;
  const lastRx = hist.rx.at(-1) ?? 0;
  const lastTx = hist.tx.at(-1) ?? 0;

  return (
    <div className="space-y-4 px-4 py-4">
      <StatBlock label="CPU" value={formatPercent(sample.cpuPercent)} hint={`peak ${formatPercent(peak)}`}>
        <Sparkline values={series} max={Math.max(peak * 1.2, 5)} className="mt-2 h-14 w-full" />
      </StatBlock>

      <StatBlock
        label="Memory"
        value={formatBytes(sample.memoryUsageBytes)}
        hint={`${formatPercent(sample.memoryPercent)} of ${formatBytes(sample.memoryLimitBytes)}`}
      >
        <Sparkline values={hist.mem} max={100} className="mt-2 h-10 w-full" tone="text-info" />
        <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-edge">
          <div
            className={`h-full rounded-full ${sample.memoryPercent > 85 ? "bg-bad" : sample.memoryPercent > 60 ? "bg-warn" : "bg-info"}`}
            style={{ width: `${Math.min(100, sample.memoryPercent)}%` }}
          />
        </div>
      </StatBlock>

      <StatBlock label="Network" value={`${formatBytesPerSec(lastRx)} ↓`} hint={`${formatBytesPerSec(lastTx)} ↑`}>
        <Sparkline values={hist.rx} className="mt-2 h-10 w-full" tone="text-ok" />
        <p className="mt-1 font-mono text-[11px] text-ink-faint">
          {formatBytes(sample.netRxBytes ?? 0)} in · {formatBytes(sample.netTxBytes ?? 0)} out
        </p>
      </StatBlock>

      <StatBlock
        label="Block I/O"
        value={formatBytes(sample.blockReadBytes ?? 0)}
        hint={`${formatBytes(sample.blockWriteBytes ?? 0)} written`}
      />

      <p className="font-mono text-[11px] text-ink-faint">1 s while this tab is open · fleet overview still samples every 3 s.</p>
    </div>
  );
}

function StatBlock({
  label,
  value,
  hint,
  children,
}: {
  label: string;
  value: string;
  hint?: string;
  children?: ReactNode;
}) {
  return (
    <div className="rounded-lg border border-edge bg-panel-2 p-3">
      <div className="flex items-baseline justify-between">
        <p className="label-caps">{label}</p>
        {hint && <p className="font-mono text-[11px] text-ink-faint">{hint}</p>}
      </div>
      <p className="mt-1 font-mono text-2xl tabular-nums text-ink">{value}</p>
      {children}
    </div>
  );
}
