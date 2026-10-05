import { useMemo, useState } from "react";
import { Boxes, Cpu, Layers, Plus, PlayCircle, StopCircle } from "lucide-react";
import { removeContainer, runContainerAction } from "../api/client";
import { ContainerDrawer, type DrawerTab } from "../components/ContainerDrawer";
import { ContainerTable } from "../components/ContainerTable";
import { CreateContainerModal } from "../components/CreateContainerModal";
import { Button } from "../components/ui/Button";
import { ConfirmDialog } from "../components/ui/ConfirmDialog";
import { useToast } from "../components/ui/toastContext";
import { ErrorBanner } from "../components/ErrorBanner";
import { StatCard } from "../components/StatCard";
import { useContainers } from "../hooks/useContainers";
import { useFleetStats } from "../hooks/useFleetStats";
import { useSearch } from "../hooks/useSearch";
import type { Container, ContainerAction, ContainerState } from "../types/domain";
import { formatBytes, formatPercent } from "../utils/format";

const PROJECT_LABEL = "com.docker.compose.project";

type StatusFilter = "all" | "running" | "stopped";
type GroupBy = "none" | "project";

const STATUS_VALUES: Record<StatusFilter, ContainerState[] | null> = {
  all: null,
  running: ["running"],
  stopped: ["exited", "dead", "created"],
};

export function ContainersPage() {
  const { data: containers, error, loading } = useContainers();
  const { query } = useSearch();
  const [selectedId, setSelectedId] = useState<string>();
  const [selectedTab, setSelectedTab] = useState<DrawerTab>("overview");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [projectFilter, setProjectFilter] = useState<string>("all");
  const [groupBy, setGroupBy] = useState<GroupBy>("none");
  const [createOpen, setCreateOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<Container>();
  const toast = useToast();

  const runningIds = useMemo(
    () => (containers ?? []).filter((c) => c.state === "running").map((c) => c.id),
    [containers],
  );
  const { statsById } = useFleetStats(runningIds);

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
    if (groupBy !== "project") return null;
    const byProject = new Map<string, Container[]>();
    for (const c of filtered) {
      const key = c.labels?.[PROJECT_LABEL] ?? "Ungrouped";
      const list = byProject.get(key) ?? [];
      list.push(c);
      byProject.set(key, list);
    }
    return [...byProject.entries()].sort(([a], [b]) => a.localeCompare(b));
  }, [filtered, groupBy]);

  const selected = containers?.find((c) => c.id === selectedId);

  const counts = useMemo(() => {
    const list = containers ?? [];
    return {
      running: list.filter((c) => c.state === "running").length,
      stopped: list.filter((c) => c.state === "exited" || c.state === "dead").length,
      total: list.length,
    };
  }, [containers]);

  const fleetTotals = useMemo(() => {
    const values = Object.values(statsById);
    return {
      cpuPercent: values.reduce((sum, s) => sum + s.cpuPercent, 0),
      memoryBytes: values.reduce((sum, s) => sum + s.memoryUsageBytes, 0),
    };
  }, [statsById]);

  const handleAction = async (id: string, action: ContainerAction) => {
    await runContainerAction(id, action);
  };

  const handleSelect = (container: Container) => {
    setSelectedTab("overview");
    setSelectedId((current) => (current === container.id ? undefined : container.id));
  };

  const handleOpenTab = (container: Container, tab: DrawerTab) => {
    setSelectedId(container.id);
    setSelectedTab(tab);
  };

  const handleRemove = (container: Container) => {
    setRemoveTarget(container);
  };

  return (
    <div className="flex flex-1 overflow-hidden">
      <main className="flex-1 overflow-y-auto px-6 py-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold text-ink">Containers</h1>
            <p className="mt-1 text-sm text-ink-muted">Manage and monitor your Docker containers.</p>
          </div>
          <Button variant="primary" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" /> New container
          </Button>
        </div>

        <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
          <StatCard label="Running" value={String(counts.running)} icon={PlayCircle} accent="emerald" />
          <StatCard label="Stopped" value={String(counts.stopped)} icon={StopCircle} accent="default" />
          <StatCard label="Total" value={String(counts.total)} icon={Boxes} accent="sky" />
          <StatCard label="CPU Usage" value={formatPercent(fleetTotals.cpuPercent)} icon={Cpu} accent="sky" />
          <StatCard label="Memory" value={formatBytes(fleetTotals.memoryBytes)} icon={Layers} accent="amber" />
        </div>

        <div className="mt-5 flex flex-wrap items-center gap-3 rounded-xl border border-edge bg-panel p-3">
          <FilterSelect
            label="Status"
            value={statusFilter}
            onChange={(v) => setStatusFilter(v as StatusFilter)}
            options={[
              { value: "all", label: "All statuses" },
              { value: "running", label: "Running" },
              { value: "stopped", label: "Stopped" },
            ]}
          />
          <FilterSelect
            label="Project"
            value={projectFilter}
            onChange={setProjectFilter}
            options={[
              { value: "all", label: "All projects" },
              ...projects.map((p) => ({ value: p, label: p })),
            ]}
          />
          <div className="ml-auto">
            <FilterSelect
              label="Group by"
              value={groupBy}
              onChange={(v) => setGroupBy(v as GroupBy)}
              options={[
                { value: "none", label: "None" },
                { value: "project", label: "Project" },
              ]}
            />
          </div>
        </div>

        {error && (
          <div className="mt-5">
            <ErrorBanner message={`Failed to load containers: ${error.message}`} />
          </div>
        )}

        <div className="mt-5 space-y-4">
          {grouped ? (
            grouped.map(([project, list]) => (
              <div key={project}>
                <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-ink-muted">
                  {project} <span className="text-ink-faint">({list.length})</span>
                </h3>
                <ContainerTable
                  containers={list}
                  statsById={statsById}
                  loading={false}
                  selectedId={selectedId}
                  onSelect={handleSelect}
                  onOpenTab={handleOpenTab}
                  onAction={handleAction}
                />
              </div>
            ))
          ) : (
            <ContainerTable
              containers={filtered}
              statsById={statsById}
              loading={loading}
              selectedId={selectedId}
              onSelect={handleSelect}
              onOpenTab={handleOpenTab}
              onAction={handleAction}
            />
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
          onRemove={handleRemove}
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

function FilterSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <label className="flex items-center gap-2 text-xs text-ink-muted">
      {label}
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="rounded-lg border border-edge bg-panel-2 px-2 py-1.5 text-sm text-ink outline-none focus:border-accent"
      >
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>
    </label>
  );
}
