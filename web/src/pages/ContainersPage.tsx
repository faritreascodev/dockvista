import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { FolderTree, Plus } from "lucide-react";
import { removeContainer, runContainerAction } from "../api/client";
import { ContainerDrawer, type DrawerTab } from "../components/ContainerDrawer";
import { ContainerTable } from "../components/ContainerTable";
import { CreateContainerModal } from "../components/CreateContainerModal";
import { ErrorBanner } from "../components/ErrorBanner";
import { PageHeader } from "../components/PageHeader";
import { StatCard } from "../components/StatCard";
import { Button } from "../components/ui/Button";
import { ConfirmDialog } from "../components/ui/ConfirmDialog";
import { Segmented } from "../components/ui/Segmented";
import { useToast } from "../components/ui/toastContext";
import { useContainers } from "../hooks/useContainers";
import { useFleetStats } from "../hooks/useFleetStats";
import { useSearch } from "../hooks/useSearch";
import { useSession } from "../hooks/useSession";
import type { Container, ContainerAction, ContainerState } from "../types/domain";
import { formatBytes, formatPercent } from "../utils/format";

const PROJECT_LABEL = "com.docker.compose.project";
const DRAWER_TABS: DrawerTab[] = ["overview", "logs", "stats", "files", "mounts", "terminal", "inspect"];

type StatusFilter = "all" | "running" | "stopped";

const STATUS_VALUES: Record<StatusFilter, ContainerState[] | null> = {
  all: null,
  running: ["running"],
  stopped: ["exited", "dead", "created"],
};

export function ContainersPage() {
  const { data: containers, error, loading } = useContainers();
  const { statsById, cpuSeriesById, totals } = useFleetStats();
  const { query } = useSearch();
  const { readOnly } = useSession();
  const [searchParams, setSearchParams] = useSearchParams();
  const [selectedId, setSelectedId] = useState<string>();
  const [selectedTab, setSelectedTab] = useState<DrawerTab>("overview");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [projectFilter, setProjectFilter] = useState<string>("all");
  const [groupByProject, setGroupByProject] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<Container>();
  const toast = useToast();

  // Deep links from the command palette and the overview:
  // /containers?focus=<id>&tab=<tab>
  useEffect(() => {
    const focus = searchParams.get("focus");
    if (!focus) return;
    const tab = searchParams.get("tab") as DrawerTab | null;
    setSelectedId(focus);
    setSelectedTab(tab && DRAWER_TABS.includes(tab) ? tab : "overview");
    setSearchParams({}, { replace: true });
  }, [searchParams, setSearchParams]);

  const projects = useMemo(() => {
    const set = new Set<string>();
    for (const c of containers ?? []) {
      const p = c.labels?.[PROJECT_LABEL];
      if (p) set.add(p);
    }
    return [...set].sort();
  }, [containers]);

  const filtered = useMemo(() => {
    let list = containers ?? [];
    const allowedStates = STATUS_VALUES[statusFilter];
    if (allowedStates) list = list.filter((c) => allowedStates.includes(c.state));
    if (projectFilter !== "all") list = list.filter((c) => c.labels?.[PROJECT_LABEL] === projectFilter);
    if (query.trim()) {
      const q = query.trim().toLowerCase();
      list = list.filter((c) => c.name.toLowerCase().includes(q) || c.image.toLowerCase().includes(q));
    }
    return list;
  }, [containers, statusFilter, projectFilter, query]);

  const grouped = useMemo(() => {
    if (!groupByProject) return null;
    const byProject = new Map<string, Container[]>();
    for (const c of filtered) {
      const key = c.labels?.[PROJECT_LABEL] ?? "Standalone";
      const list = byProject.get(key) ?? [];
      list.push(c);
      byProject.set(key, list);
    }
    return [...byProject.entries()].sort(([a], [b]) => a.localeCompare(b));
  }, [filtered, groupByProject]);

  // Deep links may carry a short ID or a name, the same forms the docker
  // CLI accepts.
  const selected = selectedId
    ? (containers?.find((c) => c.id === selectedId) ??
      containers?.find((c) => c.name === selectedId || (selectedId.length >= 4 && c.id.startsWith(selectedId))))
    : undefined;

  const counts = useMemo(() => {
    const list = containers ?? [];
    return {
      all: list.length,
      running: list.filter((c) => STATUS_VALUES.running!.includes(c.state)).length,
      stopped: list.filter((c) => STATUS_VALUES.stopped!.includes(c.state)).length,
    };
  }, [containers]);

  const handleAction = async (id: string, action: ContainerAction) => {
    try {
      await runContainerAction(id, action);
    } catch (err) {
      toast.push("error", err instanceof Error ? err.message : `Failed to ${action} container.`);
    }
  };

  const handleSelect = (container: Container) => {
    setSelectedTab("overview");
    setSelectedId(selected?.id === container.id ? undefined : container.id);
  };

  const handleOpenTab = (container: Container, tab: DrawerTab) => {
    setSelectedId(container.id);
    setSelectedTab(tab);
  };

  const tableProps = { statsById, cpuSeriesById, selectedId: selected?.id, onSelect: handleSelect, onOpenTab: handleOpenTab, onAction: handleAction };

  return (
    <div className="flex min-h-0 flex-1 overflow-hidden">
      <main className="min-w-0 flex-1 overflow-y-auto px-4 py-6 md:px-8">
        <PageHeader
          kicker="Workloads"
          title="Containers"
          description="Every container on this engine, with live CPU and memory."
          actions={
            !readOnly && (
              <Button variant="primary" onClick={() => setCreateOpen(true)}>
                <Plus className="h-4 w-4" /> New container
              </Button>
            )
          }
        />

        <div className="mt-6 grid grid-cols-2 gap-3 lg:grid-cols-4">
          <StatCard label="Running" tone="ok" value={counts.running} hint={`of ${counts.all} containers`} />
          <StatCard label="Stopped" tone="muted" value={counts.stopped} hint="exited, dead, or created" />
          <StatCard label="CPU" tone="accent" value={formatPercent(totals.cpu.at(-1) ?? 0)} series={totals.cpu} />
          <StatCard
            label="Memory"
            tone="info"
            value={formatBytes(totals.memoryBytes.at(-1) ?? 0)}
            series={totals.memoryBytes}
          />
        </div>

        <div className="mt-6 flex flex-wrap items-center gap-2">
          <Segmented
            value={statusFilter}
            onChange={setStatusFilter}
            options={[
              { value: "all", label: "All", count: counts.all },
              { value: "running", label: "Running", count: counts.running },
              { value: "stopped", label: "Stopped", count: counts.stopped },
            ]}
          />
          {projects.length > 0 && (
            <select
              value={projectFilter}
              onChange={(e) => setProjectFilter(e.target.value)}
              aria-label="Filter by compose project"
              className="h-8 rounded-md border border-edge bg-panel px-2 text-sm text-ink outline-none focus:border-accent"
            >
              <option value="all">All projects</option>
              {projects.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          )}
          <button
            onClick={() => setGroupByProject((v) => !v)}
            aria-pressed={groupByProject}
            className={`ml-auto flex h-8 items-center gap-1.5 rounded-md border px-2.5 text-xs font-medium transition ${
              groupByProject ? "border-accent/50 bg-accent/10 text-accent" : "border-edge bg-panel text-ink-muted hover:text-ink"
            }`}
          >
            <FolderTree className="h-3.5 w-3.5" /> Group by project
          </button>
        </div>

        {error && (
          <div className="mt-4">
            <ErrorBanner message={`Failed to load containers: ${error.message}`} />
          </div>
        )}

        <div className="mt-4 space-y-5">
          {grouped ? (
            grouped.map(([project, list]) => (
              <section key={project}>
                <h3 className="mb-2 flex items-center gap-2 text-sm font-medium text-ink">
                  {project}
                  <span className="font-mono text-[11px] text-ink-faint">{list.length}</span>
                </h3>
                <ContainerTable containers={list} loading={false} {...tableProps} />
              </section>
            ))
          ) : (
            <ContainerTable containers={containers ? filtered : undefined} loading={loading} {...tableProps} />
          )}
        </div>
      </main>

      {selected && (
        <ContainerDrawer
          container={selected}
          stats={statsById[selected.id]}
          tab={selectedTab}
          onTabChange={setSelectedTab}
          onClose={() => setSelectedId(undefined)}
          onAction={handleAction}
          onRemove={setRemoveTarget}
        />
      )}

      {createOpen && <CreateContainerModal onClose={() => setCreateOpen(false)} onCreated={() => {}} />}

      {removeTarget && (
        <ConfirmDialog
          title="Remove container"
          description={`This will permanently remove "${removeTarget.name}"${removeTarget.state === "running" ? " (it's currently running and will be force-stopped first)" : ""}. This cannot be undone.`}
          confirmLabel="Remove"
          onConfirm={async () => {
            await removeContainer(removeTarget.id, true);
            setSelectedId((current) => (current === removeTarget.id ? undefined : current));
            toast.push("success", `Removed "${removeTarget.name}".`);
          }}
          onClose={() => setRemoveTarget(undefined)}
        />
      )}
    </div>
  );
}
