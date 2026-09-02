import { useMemo } from "react";
import { Layers } from "lucide-react";
import { runContainerAction } from "../api/client";
import { ContainerActions } from "../components/ContainerActions";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { StatusBadge } from "../components/StatusBadge";
import { TableSkeleton } from "../components/Skeleton";
import { useContainers } from "../hooks/useContainers";
import { useSearch } from "../hooks/useSearchContext";
import type { Container, ContainerAction } from "../types/domain";
import { shortId } from "../utils/format";

const PROJECT_LABEL = "com.docker.compose.project";
const SERVICE_LABEL = "com.docker.compose.service";

interface Project {
  name: string;
  services: Container[];
}

function groupByProject(containers: Container[]): Project[] {
  const byProject = new Map<string, Container[]>();
  for (const c of containers) {
    const project = c.labels?.[PROJECT_LABEL];
    if (!project) continue;
    const list = byProject.get(project) ?? [];
    list.push(c);
    byProject.set(project, list);
  }
  return [...byProject.entries()]
    .map(([name, services]) => ({ name, services }))
    .sort((a, b) => a.name.localeCompare(b.name));
}

/**
 * Read-only grouping of already-known containers by their
 * com.docker.compose.project label — not full `docker compose up/down`
 * orchestration, which would need host filesystem access to compose files
 * DockVista (daemon-socket-only) doesn't have. See README roadmap.
 */
export function ComposePage() {
  const { data: containers, error, loading } = useContainers();
  const { query } = useSearch();

  const projects = useMemo(() => groupByProject(containers ?? []), [containers]);
  const filtered = useMemo(() => {
    if (!query.trim()) return projects;
    const q = query.trim().toLowerCase();
    return projects
      .map((p) => ({
        ...p,
        services: p.services.filter(
          (s) => s.name.toLowerCase().includes(q) || p.name.toLowerCase().includes(q),
        ),
      }))
      .filter((p) => p.services.length > 0 || p.name.toLowerCase().includes(q));
  }, [projects, query]);

  const handleAction = async (id: string, action: ContainerAction) => {
    await runContainerAction(id, action);
  };

  return (
    <main className="flex-1 overflow-y-auto px-6 py-6">
      <h1 className="text-xl font-semibold text-ink">Compose</h1>
      <p className="mt-1 text-sm text-ink-muted">
        Containers grouped by their Compose project. Start/stop each service, or use the actions per row —
        orchestrating whole stacks from compose files isn't supported yet.
      </p>

      {error && (
        <div className="mt-5">
          <ErrorBanner message={`Failed to load containers: ${error.message}`} />
        </div>
      )}

      <div className="mt-5 space-y-4">
        {loading && !containers ? (
          <div className="rounded-xl border border-edge bg-panel">
            <TableSkeleton />
          </div>
        ) : filtered.length === 0 ? (
          <div className="rounded-xl border border-edge bg-panel">
            <EmptyState message="No Compose projects found among your containers." />
          </div>
        ) : (
          filtered.map((project) => (
            <div key={project.name} className="overflow-hidden rounded-xl border border-edge bg-panel">
              <div className="flex items-center gap-2 border-b border-edge bg-panel-2 px-4 py-2.5">
                <Layers className="h-4 w-4 text-accent" />
                <h2 className="text-sm font-semibold text-ink">{project.name}</h2>
                <span className="text-xs text-ink-faint">
                  {project.services.length} service{project.services.length === 1 ? "" : "s"}
                </span>
              </div>
              <div className="divide-y divide-edge">
                {project.services.map((service) => (
                  <div key={service.id} className="flex items-center justify-between gap-4 px-4 py-2.5">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-ink">
                        {service.labels?.[SERVICE_LABEL] ?? service.name}
                      </p>
                      <p className="truncate font-mono text-xs text-ink-faint">{shortId(service.id)}</p>
                    </div>
                    <div className="flex shrink-0 items-center gap-3">
                      <StatusBadge state={service.state} />
                      <ContainerActions container={service} onAction={handleAction} />
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))
        )}
      </div>
    </main>
  );
}
