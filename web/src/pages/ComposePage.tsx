import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Layers, Play, RefreshCw, Square, Upload } from "lucide-react";
import { createStack, createStackFromGit, deleteStack, downStack, listStacks, runContainerAction, syncStack, upStack } from "../api/client";
import { ContainerActions } from "../components/ContainerActions";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { PageHeader } from "../components/PageHeader";
import { TableSkeleton } from "../components/Skeleton";
import { StatusBadge } from "../components/StatusBadge";
import { Button } from "../components/ui/Button";
import { FieldLabel } from "../components/ui/Form";
import { Segmented } from "../components/ui/Segmented";
import { useToast } from "../components/ui/toastContext";
import { useContainers } from "../hooks/useContainers";
import { useFleetStats } from "../hooks/useFleetStats";
import { useSearch } from "../hooks/useSearch";
import { useSession } from "../hooks/useSession";
import type { Container, ContainerAction, Stack } from "../types/domain";
import { formatBytes, formatPercent } from "../utils/format";

const PROJECT_LABEL = "com.docker.compose.project";
const SERVICE_LABEL = "com.docker.compose.service";
const WORKDIR_LABEL = "com.docker.compose.project.working_dir";

interface Project {
  name: string;
  workingDir?: string | undefined;
  services: Container[];
}

function groupByProject(containers: Container[]): Project[] {
  const byProject = new Map<string, Project>();
  for (const c of containers) {
    const name = c.labels?.[PROJECT_LABEL];
    if (!name) continue;
    const project: Project = byProject.get(name) ?? { name, workingDir: c.labels?.[WORKDIR_LABEL], services: [] };
    project.services.push(c);
    byProject.set(name, project);
  }
  return [...byProject.values()]
    .map((p) => ({ ...p, services: p.services.sort((a, b) => serviceName(a).localeCompare(serviceName(b))) }))
    .sort((a, b) => a.name.localeCompare(b.name));
}

function serviceName(c: Container): string {
  return c.labels?.[SERVICE_LABEL] ?? c.name;
}

/**
 * Workspace stacks (paste YAML or clone https git, up/down through the
 * Docker API) plus grouping of already-known containers by
 * com.docker.compose.project. Per-project Start all / Stop all still fans
 * out to container endpoints.
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
        services: p.name.toLowerCase().includes(q)
          ? p.services
          : p.services.filter((s) => serviceName(s).toLowerCase().includes(q) || s.name.toLowerCase().includes(q)),
      }))
      .filter((p) => p.services.length > 0);
  }, [projects, query]);

  return (
    <main className="flex-1 overflow-y-auto px-4 py-6 md:px-8">
      <PageHeader
        kicker="Workloads"
        title="Compose"
        description="Stacks DockVista owns (paste YAML or clone https git into the data dir, then up/down), plus containers already labelled by Compose on this engine."
      />

      <StacksPanel />

      {error && (
        <div className="mt-5">
          <ErrorBanner message={`Failed to load containers: ${error.message}`} />
        </div>
      )}

      <div className="mt-6 grid gap-4 2xl:grid-cols-2">
        {loading && !containers ? (
          <div className="rounded-lg border border-edge bg-panel 2xl:col-span-2">
            <TableSkeleton />
          </div>
        ) : filtered.length === 0 ? (
          <div className="rounded-lg border border-edge bg-panel 2xl:col-span-2">
            <EmptyState
              icon={Layers}
              message={query ? "No projects or services match your filter." : "No Compose projects found among your containers."}
            />
          </div>
        ) : (
          filtered.map((project) => <ProjectCard key={project.name} project={project} />)
        )}
      </div>
    </main>
  );
}

function StacksPanel() {
  const { readOnly } = useSession();
  const toast = useToast();
  const [stacks, setStacks] = useState<Stack[]>([]);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [mode, setMode] = useState<"paste" | "git">("paste");
  const [yaml, setYaml] = useState("services:\n  web:\n    image: nginx:alpine\n    ports:\n      - \"8080:80\"\n");
  const [gitUrl, setGitUrl] = useState("");
  const [gitRef, setGitRef] = useState("main");
  const [composeFile, setComposeFile] = useState("");
  const [gitUsername, setGitUsername] = useState("");
  const [gitToken, setGitToken] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = async () => setStacks(await listStacks());
  useEffect(() => {
    void refresh();
  }, []);

  return (
    <section className="mt-6 rounded-lg border border-edge bg-panel">
      <div className="flex items-center justify-between border-b border-edge px-5 py-3">
        <div>
          <h2 className="text-sm font-semibold text-ink">Workspace stacks</h2>
          <p className="text-xs text-ink-faint">
            YAML or an https clone lives under the data dir. Bind mounts must stay inside that folder. No SSH remotes.
          </p>
        </div>
        {!readOnly && (
          <Button variant="primary" onClick={() => setOpen(true)}>
            <Upload className="h-4 w-4" /> New stack
          </Button>
        )}
      </div>
      <div className="divide-y divide-edge">
        {stacks.length === 0 ? (
          <p className="px-5 py-4 text-sm text-ink-faint">No stacks yet.</p>
        ) : (
          stacks.map((s) => (
            <div key={s.id} className="flex items-center justify-between gap-3 px-5 py-3">
              <div>
                <p className="font-mono text-sm text-ink">{s.name}</p>
                <p className="text-[11px] text-ink-faint">
                  project dv-{s.name}
                  {s.gitUrl ? ` · ${s.gitRef || "main"}` : ""}
                </p>
                {s.gitUrl && <p className="truncate font-mono text-[11px] text-ink-faint">{s.gitUrl}</p>}
              </div>
              {!readOnly && (
                <div className="flex gap-2">
                  {s.gitUrl && (
                    <Button
                      size="sm"
                      onClick={async () => {
                        try {
                          await syncStack(s.id);
                          toast.push("success", `Synced ${s.name}.`);
                          await refresh();
                        } catch (err) {
                          toast.push("error", err instanceof Error ? err.message : `Could not sync ${s.name}.`);
                        }
                      }}
                    >
                      <RefreshCw className="h-3 w-3" /> Sync
                    </Button>
                  )}
                  <Button
                    size="sm"
                    onClick={async () => {
                      try {
                        await upStack(s.id);
                        toast.push("success", `Brought up ${s.name}.`);
                      } catch (err) {
                        toast.push("error", err instanceof Error ? err.message : `Could not bring up ${s.name}.`);
                      }
                    }}
                  >
                    Up
                  </Button>
                  <Button
                    size="sm"
                    onClick={async () => {
                      try {
                        await downStack(s.id);
                        toast.push("success", `Took down ${s.name}.`);
                      } catch (err) {
                        toast.push("error", err instanceof Error ? err.message : `Could not take down ${s.name}.`);
                      }
                    }}
                  >
                    Down
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={async () => {
                      try {
                        await deleteStack(s.id);
                        await refresh();
                      } catch (err) {
                        toast.push("error", err instanceof Error ? err.message : `Could not delete ${s.name}.`);
                      }
                    }}
                  >
                    Delete
                  </Button>
                </div>
              )}
            </div>
          ))
        )}
      </div>
      {open && (
        <div className="border-t border-edge px-5 py-4">
          <FieldLabel htmlFor="stack-name">Name (lowercase)</FieldLabel>
          <input id="stack-name" className="field mt-1" value={name} onChange={(e) => setName(e.target.value)} />
          <div className="mt-3">
            <Segmented
              value={mode}
              onChange={setMode}
              options={[
                { value: "paste", label: "Paste YAML" },
                { value: "git", label: "From git" },
              ]}
            />
          </div>
          {mode === "paste" ? (
            <>
              <div className="mt-3">
                <FieldLabel htmlFor="stack-yaml">compose.yml</FieldLabel>
              </div>
              <textarea id="stack-yaml" className="field mt-1 font-mono" rows={10} value={yaml} onChange={(e) => setYaml(e.target.value)} />
            </>
          ) : (
            <div className="mt-3 space-y-3">
              <div>
                <FieldLabel htmlFor="stack-git-url" hint="https only">
                  Repository
                </FieldLabel>
                <input
                  id="stack-git-url"
                  className="field mt-1 font-mono"
                  placeholder="https://github.com/org/repo.git"
                  value={gitUrl}
                  onChange={(e) => setGitUrl(e.target.value)}
                />
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <div>
                  <FieldLabel htmlFor="stack-git-ref">Branch or tag</FieldLabel>
                  <input id="stack-git-ref" className="field mt-1 font-mono" value={gitRef} onChange={(e) => setGitRef(e.target.value)} />
                </div>
                <div>
                  <FieldLabel htmlFor="stack-compose-file" hint="optional">
                    Compose path
                  </FieldLabel>
                  <input
                    id="stack-compose-file"
                    className="field mt-1 font-mono"
                    placeholder="compose.yml"
                    value={composeFile}
                    onChange={(e) => setComposeFile(e.target.value)}
                  />
                </div>
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <input className="field" placeholder="username (private repos)" value={gitUsername} onChange={(e) => setGitUsername(e.target.value)} />
                <input
                  className="field"
                  type="password"
                  placeholder="token (stored 0600 in the data dir)"
                  value={gitToken}
                  onChange={(e) => setGitToken(e.target.value)}
                />
              </div>
            </div>
          )}
          <div className="mt-3 flex gap-2">
            <Button
              variant="primary"
              loading={busy}
              onClick={async () => {
                setBusy(true);
                try {
                  if (mode === "git") {
                    await createStackFromGit({
                      name,
                      gitUrl,
                      ...(gitRef.trim() ? { gitRef: gitRef.trim() } : {}),
                      ...(composeFile.trim() ? { composeFile: composeFile.trim() } : {}),
                      ...(gitUsername.trim() ? { gitUsername: gitUsername.trim() } : {}),
                      ...(gitToken.trim() ? { gitToken: gitToken.trim() } : {}),
                    });
                    toast.push("success", `Cloned ${name}.`);
                  } else {
                    await createStack(name, yaml);
                    toast.push("success", `Saved ${name}.`);
                  }
                  setOpen(false);
                  await refresh();
                } catch (err) {
                  toast.push("error", err instanceof Error ? err.message : "Could not save stack.");
                } finally {
                  setBusy(false);
                }
              }}
            >
              {mode === "git" ? "Clone" : "Save"}
            </Button>
            <Button variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
          </div>
        </div>
      )}
    </section>
  );
}

function ProjectCard({ project }: { project: Project }) {
  const { readOnly } = useSession();
  const { statsById } = useFleetStats();
  const toast = useToast();
  const [busy, setBusy] = useState<"start" | "stop" | null>(null);

  const running = project.services.filter((s) => s.state === "running").length;
  const total = project.services.length;
  const health = running === total ? "ok" : running === 0 ? "idle" : "partial";
  const cpu = project.services.reduce((sum, s) => sum + (statsById[s.id]?.cpuPercent ?? 0), 0);
  const mem = project.services.reduce((sum, s) => sum + (statsById[s.id]?.memoryUsageBytes ?? 0), 0);

  const handleAction = async (id: string, action: ContainerAction) => {
    try {
      await runContainerAction(id, action);
    } catch (err) {
      toast.push("error", err instanceof Error ? err.message : `Failed to ${action} container.`);
    }
  };

  const runAll = async (kind: "start" | "stop") => {
    const targets = project.services.filter((s) =>
      kind === "start" ? ["exited", "created", "dead"].includes(s.state) : ["running", "paused", "restarting"].includes(s.state),
    );
    setBusy(kind);
    const results = await Promise.allSettled(targets.map((s) => runContainerAction(s.id, kind)));
    setBusy(null);
    const failed = results.filter((r) => r.status === "rejected").length;
    if (failed > 0) toast.push("error", `${failed} of ${targets.length} services failed to ${kind}.`);
    else toast.push("success", `${kind === "start" ? "Started" : "Stopped"} ${targets.length} service${targets.length === 1 ? "" : "s"} in ${project.name}.`);
  };

  return (
    <section className="overflow-hidden rounded-lg border border-edge bg-panel">
      <div className="border-b border-edge px-4 py-3">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <Layers className="h-4 w-4 text-accent" />
              <h2 className="truncate font-semibold text-ink">{project.name}</h2>
              <span
                className={`rounded px-1.5 font-mono text-[10.5px] ${
                  health === "ok" ? "bg-ok/10 text-ok" : health === "partial" ? "bg-warn/10 text-warn" : "bg-panel-2 text-ink-faint"
                }`}
              >
                {running}/{total} up
              </span>
            </div>
            {project.workingDir && (
              <p className="mt-0.5 truncate font-mono text-[11px] text-ink-faint" title={project.workingDir}>
                {project.workingDir}
              </p>
            )}
          </div>
          {!readOnly && (
            <div className="flex shrink-0 gap-1.5">
              <Button size="sm" onClick={() => void runAll("start")} loading={busy === "start"} disabled={busy !== null || running === total}>
                <Play className="h-3 w-3" /> Start all
              </Button>
              <Button size="sm" onClick={() => void runAll("stop")} loading={busy === "stop"} disabled={busy !== null || running === 0}>
                <Square className="h-3 w-3" /> Stop all
              </Button>
            </div>
          )}
        </div>

        <div className="mt-3 flex items-center gap-3">
          <div className="flex h-1.5 flex-1 gap-0.5">
            {project.services.map((s) => (
              <span
                key={s.id}
                title={`${serviceName(s)}: ${s.state}`}
                className={`h-full flex-1 rounded-sm ${s.state === "running" ? "bg-ok" : s.state === "paused" || s.state === "restarting" ? "bg-warn" : s.state === "dead" ? "bg-bad" : "bg-edge-strong"}`}
              />
            ))}
          </div>
          {running > 0 && (
            <span className="shrink-0 font-mono text-[11px] text-ink-muted">
              {formatPercent(cpu)} · {formatBytes(mem)}
            </span>
          )}
        </div>
      </div>

      <ul className="divide-y divide-edge">
        {project.services.map((service) => (
          <li key={service.id} className="flex items-center justify-between gap-4 px-4 py-2.5">
            <Link to={`/containers?focus=${encodeURIComponent(service.id)}`} className="group min-w-0">
              <p className="truncate text-sm font-medium text-ink group-hover:text-accent">{serviceName(service)}</p>
              <p className="truncate font-mono text-[11px] text-ink-faint">{service.image}</p>
            </Link>
            <div className="flex shrink-0 items-center gap-3">
              <StatusBadge state={service.state} />
              <ContainerActions container={service} onAction={handleAction} />
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}
