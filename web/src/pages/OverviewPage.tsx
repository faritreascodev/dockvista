import { useMemo } from "react";
import { Link } from "react-router-dom";
import { ArrowUpRight, Radio } from "lucide-react";
import { ErrorBanner } from "../components/ErrorBanner";
import { PageHeader } from "../components/PageHeader";
import { Sparkline } from "../components/Sparkline";
import { StatCard } from "../components/StatCard";
import { useContainers } from "../hooks/useContainers";
import { useFleetStats } from "../hooks/useFleetStats";
import { useRecentEvents } from "../hooks/resourceRefresh";
import { useDiskUsage, useSystemInfo } from "../hooks/useSystem";
import type { DiskUsage, DiskUsageCategory, DockerEvent } from "../types/domain";
import { formatBytes, formatPercent } from "../utils/format";

export function OverviewPage() {
  const { data: info, error: infoError } = useSystemInfo();
  const { data: disk, error: diskError } = useDiskUsage();
  const { totals } = useFleetStats();

  const cpuNow = totals.cpu.at(-1) ?? 0;
  const memNow = totals.memoryBytes.at(-1) ?? 0;
  const cpuCapacity = (info?.cpus ?? 1) * 100;

  return (
    <main className="flex-1 overflow-y-auto">
      <div className="grid-paper border-b border-edge">
        <div className="bg-gradient-to-b from-canvas/40 to-canvas px-4 pb-6 pt-6 md:px-8">
          <PageHeader
            kicker={info ? `host · ${info.name}` : "host"}
            title="Overview"
            description={
              info ? (
                <span className="font-mono text-xs">
                  {info.operatingSystem} · {info.architecture} · kernel {info.kernelVersion}
                </span>
              ) : (
                "Live state of the Docker engine this console is attached to."
              )
            }
          />

          <div className="mt-6 grid grid-cols-2 gap-3 xl:grid-cols-4">
            <StatCard
              label="Running"
              tone="ok"
              value={
                info ? (
                  <>
                    {info.containersRunning}
                    <span className="text-base text-ink-faint">/{info.containers}</span>
                  </>
                ) : (
                  "--"
                )
              }
              hint={info ? `${info.containersPaused} paused · ${info.containersStopped} stopped` : undefined}
            />
            <StatCard
              label="Fleet CPU"
              tone="accent"
              value={formatPercent(cpuNow)}
              hint={info ? `of ${info.cpus} cores (${cpuCapacity}%)` : undefined}
              series={totals.cpu}
              seriesMax={Math.max(5, ...totals.cpu) * 1.25}
            />
            <StatCard
              label="Fleet memory"
              tone="info"
              value={formatBytes(memNow)}
              hint={info ? `of ${formatBytes(info.memoryBytes)} host memory` : undefined}
              series={totals.memoryBytes}
            />
            <StatCard
              label="Disk used"
              tone="warn"
              value={disk ? formatBytes(totalSize(disk)) : "--"}
              hint={disk ? `${formatBytes(totalReclaimable(disk))} reclaimable` : "Computing…"}
            />
          </div>
        </div>
      </div>

      <div className="space-y-5 px-4 py-6 md:px-8">
        {(infoError || diskError) && (
          <ErrorBanner message={`Some engine data failed to load: ${(infoError ?? diskError)?.message}`} />
        )}

        <div className="grid gap-5 xl:grid-cols-5">
          <Panel title="Disk usage" kicker="docker system df" className="xl:col-span-3">
            <DiskBreakdown disk={disk} />
          </Panel>
          <Panel
            title="Top consumers"
            kicker="by cpu, live"
            className="xl:col-span-2"
            action={
              <Link to="/containers" className="flex items-center gap-1 text-xs text-ink-muted hover:text-accent">
                All containers <ArrowUpRight className="h-3 w-3" />
              </Link>
            }
          >
            <TopConsumers />
          </Panel>
        </div>

        <div className="grid gap-5 xl:grid-cols-5">
          <Panel title="Live activity" kicker="engine events" className="xl:col-span-3">
            <ActivityFeed />
          </Panel>
          <Panel title="Engine" kicker="docker info" className="xl:col-span-2">
            {info ? (
              <dl className="divide-y divide-edge">
                {(
                  [
                    ["Server version", info.serverVersion],
                    ["Storage driver", info.storageDriver],
                    ["OS type", `${info.osType}/${info.architecture}`],
                    ["CPUs", String(info.cpus)],
                    ["Memory", formatBytes(info.memoryBytes)],
                    ["Images", String(info.images)],
                  ] as const
                ).map(([k, v]) => (
                  <div key={k} className="flex items-center justify-between gap-4 py-2.5 text-sm">
                    <dt className="text-ink-muted">{k}</dt>
                    <dd className="truncate font-mono text-xs text-ink">{v}</dd>
                  </div>
                ))}
              </dl>
            ) : (
              <SkeletonLines rows={6} />
            )}
          </Panel>
        </div>
      </div>
    </main>
  );
}

function Panel({
  title,
  kicker,
  action,
  className = "",
  children,
}: {
  title: string;
  kicker: string;
  action?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <section className={`rounded-lg border border-edge bg-panel ${className}`}>
      <div className="flex items-center justify-between gap-3 border-b border-edge px-4 py-3">
        <div className="flex items-baseline gap-2.5">
          <h2 className="text-sm font-semibold text-ink">{title}</h2>
          <span className="label-caps">{kicker}</span>
        </div>
        {action}
      </div>
      <div className="px-4 py-3">{children}</div>
    </section>
  );
}

function SkeletonLines({ rows }: { rows: number }) {
  return (
    <div className="space-y-3 py-1">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="h-4 animate-pulse rounded bg-panel-2" />
      ))}
    </div>
  );
}

const CATEGORIES: { key: keyof DiskUsage; label: string; bar: string; to?: string }[] = [
  { key: "images", label: "Images", bar: "bg-accent", to: "/images" },
  { key: "containers", label: "Containers", bar: "bg-info", to: "/containers" },
  { key: "volumes", label: "Volumes", bar: "bg-ok", to: "/volumes" },
  { key: "buildCache", label: "Build cache", bar: "bg-ink-faint" },
];

function totalSize(d: DiskUsage) {
  return d.images.sizeBytes + d.containers.sizeBytes + d.volumes.sizeBytes + d.buildCache.sizeBytes;
}

function totalReclaimable(d: DiskUsage) {
  return d.images.reclaimableBytes + d.containers.reclaimableBytes + d.volumes.reclaimableBytes + d.buildCache.reclaimableBytes;
}

function DiskBreakdown({ disk }: { disk: DiskUsage | undefined }) {
  if (!disk) return <SkeletonLines rows={5} />;
  const total = Math.max(totalSize(disk), 1);

  return (
    <div>
      <div className="flex h-2.5 w-full overflow-hidden rounded-sm bg-panel-2">
        {CATEGORIES.map((c) => (
          <div
            key={c.key}
            className={`${c.bar} h-full border-r border-panel last:border-r-0`}
            style={{ width: `${(disk[c.key].sizeBytes / total) * 100}%` }}
            title={`${c.label}: ${formatBytes(disk[c.key].sizeBytes)}`}
          />
        ))}
      </div>

      <table className="mt-3 w-full text-sm">
        <thead>
          <tr className="text-left">
            <th className="label-caps py-2 font-medium">Type</th>
            <th className="label-caps py-2 text-right font-medium">Count</th>
            <th className="label-caps py-2 text-right font-medium">Size</th>
            <th className="label-caps py-2 text-right font-medium">Reclaimable</th>
          </tr>
        </thead>
        <tbody>
          {CATEGORIES.map((c) => {
            const cat: DiskUsageCategory = disk[c.key];
            const reclaimPct = cat.sizeBytes > 0 ? (cat.reclaimableBytes / cat.sizeBytes) * 100 : 0;
            const label = (
              <span className="flex items-center gap-2">
                <span className={`h-2 w-2 rounded-[2px] ${c.bar}`} />
                {c.label}
              </span>
            );
            return (
              <tr key={c.key} className="border-t border-edge">
                <td className="py-2.5 text-ink">
                  {c.to ? (
                    <Link to={c.to} className="hover:text-accent">
                      {label}
                    </Link>
                  ) : (
                    label
                  )}
                </td>
                <td className="py-2.5 text-right font-mono text-xs text-ink-muted">
                  {cat.count}
                  <span className="text-ink-faint"> ({cat.active} active)</span>
                </td>
                <td className="py-2.5 text-right font-mono text-xs text-ink">{formatBytes(cat.sizeBytes)}</td>
                <td className="py-2.5 text-right font-mono text-xs">
                  <span className={cat.reclaimableBytes > 0 ? "text-warn" : "text-ink-faint"}>
                    {formatBytes(cat.reclaimableBytes)}
                  </span>
                  {cat.reclaimableBytes > 0 && <span className="text-ink-faint"> ({reclaimPct.toFixed(0)}%)</span>}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function TopConsumers() {
  const { data: containers } = useContainers();
  const { statsById, cpuSeriesById } = useFleetStats();

  const rows = useMemo(() => {
    const names = new Map((containers ?? []).map((c) => [c.id, c.name]));
    return Object.values(statsById)
      .sort((a, b) => b.cpuPercent - a.cpuPercent)
      .slice(0, 6)
      .map((s) => ({ ...s, name: names.get(s.containerId) ?? s.containerId.slice(0, 12) }));
  }, [containers, statsById]);

  if (rows.length === 0) {
    return <p className="py-6 text-center text-sm text-ink-faint">No running containers.</p>;
  }

  return (
    <ul className="divide-y divide-edge">
      {rows.map((r) => (
        <li key={r.containerId}>
          <Link
            to={`/containers?focus=${encodeURIComponent(r.containerId)}&tab=stats`}
            className="group flex items-center gap-3 py-2.5"
          >
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-ink group-hover:text-accent">{r.name}</p>
              <p className="font-mono text-[11px] text-ink-faint">{formatBytes(r.memoryUsageBytes)} mem</p>
            </div>
            <Sparkline
              values={cpuSeriesById[r.containerId] ?? []}
              max={Math.max(5, ...(cpuSeriesById[r.containerId] ?? []))}
              className="h-7 w-24"
            />
            <span className="w-14 text-right font-mono text-xs tabular-nums text-ink">{formatPercent(r.cpuPercent)}</span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

const EVENT_TONE: Record<string, string> = {
  start: "text-ok",
  unpause: "text-ok",
  create: "text-info",
  pull: "text-info",
  die: "text-bad",
  kill: "text-bad",
  oom: "text-bad",
  destroy: "text-bad",
  delete: "text-bad",
  stop: "text-warn",
  pause: "text-warn",
  restart: "text-warn",
};

function eventSubject(e: DockerEvent): string {
  return e.attributes?.name ?? e.attributes?.image ?? e.actorId.replace(/^sha256:/, "").slice(0, 12);
}

function ActivityFeed() {
  const events = useRecentEvents();

  if (events.length === 0) {
    return (
      <div className="flex flex-col items-center gap-2 py-8 text-center">
        <Radio className="h-5 w-5 text-ink-faint" />
        <p className="text-sm text-ink-muted">Listening for engine events…</p>
        <p className="text-xs text-ink-faint">Start, stop, or pull something and it shows up here instantly.</p>
      </div>
    );
  }

  return (
    <ol className="max-h-80 space-y-px overflow-y-auto font-mono text-xs">
      {events.map((e, i) => {
        const action = e.action.split(":")[0] ?? e.action;
        return (
          <li key={`${e.time}-${e.actorId}-${i}`} className="flex items-center gap-3 rounded px-1.5 py-1.5 hover:bg-panel-2 animate-fade-in">
            <time className="w-16 shrink-0 text-ink-faint">{new Date(e.time).toLocaleTimeString([], { hour12: false })}</time>
            <span className="w-16 shrink-0 text-ink-muted">{e.type}</span>
            <span className={`w-20 shrink-0 font-medium ${EVENT_TONE[action] ?? "text-ink"}`}>{action}</span>
            <span className="truncate text-ink">{eventSubject(e)}</span>
          </li>
        );
      })}
    </ol>
  );
}
