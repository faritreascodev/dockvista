import type {
  Container,
  ContainerAction,
  ContainerStats,
  ContainerFS,
  DirListing,
  DockerImage,
  DockerNetwork,
  DockerVolume,
  CleanupPlan,
  CleanupReport,
  DiskUsage,
  EngineInfo,
  FSChangeList,
  FSEntry,
  StorageInventory,
  PruneResult,
  SystemInfo,
  Environment,
  Stack,
  SwarmSnapshot,
  Registry,
  ImageLayer,
} from "../types/domain";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    // Parsed JSON error body, when the server sent one. Some error responses
    // carry data the UI needs (e.g. the id of a container that was created).
    public body: unknown = null,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { credentials: "include", ...init });
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new ApiError(res.status, body?.error ?? `request failed: ${res.status}`, body);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}

export interface AuthStatus {
  initialized: boolean;
}

export type UserRole = "admin" | "operator" | "viewer";

export interface AuthUser {
  username: string;
  role: UserRole;
  // True when this session cannot mutate Docker: process flag or Viewer.
  readOnly: boolean;
  // True when the process itself was started with DOCKVISTA_READ_ONLY.
  instanceReadOnly: boolean;
}

export interface AccountUser {
  username: string;
  role: UserRole;
  createdAt: string;
}

export interface InviteRecord {
  id: string;
  role: UserRole;
  createdBy: string;
  expiresAt: string;
  used: boolean;
}

export interface CreatedInvite {
  id: string;
  role: UserRole;
  expiresAt: string;
  token: string;
  path: string;
}

export interface InvitePeek {
  role: UserRole;
  expiresAt: string;
}

export function getAuthStatus(): Promise<AuthStatus> {
  return request<AuthStatus>("/api/auth/status");
}

export function setupAdmin(username: string, password: string, setupToken: string): Promise<void> {
  return request<{ status: string }>("/api/auth/setup", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password, setupToken }),
  }).then(() => undefined);
}

export function login(username: string, password: string): Promise<AuthUser> {
  return request<AuthUser>("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
}

export function logout(): Promise<void> {
  return request<void>("/api/auth/logout", { method: "POST" });
}

export function getMe(): Promise<AuthUser> {
  return request<AuthUser>("/api/auth/me");
}

export function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  return request<{ status: string }>("/api/auth/password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ currentPassword, newPassword }),
  }).then(() => undefined);
}

export function peekInvite(token: string): Promise<InvitePeek> {
  return request<InvitePeek>(`/api/auth/invite?token=${encodeURIComponent(token)}`);
}

export function acceptInvite(token: string, username: string, password: string): Promise<void> {
  return request<{ status: string }>("/api/auth/accept", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token, username, password }),
  }).then(() => undefined);
}

export function listUsers(): Promise<AccountUser[]> {
  return request<AccountUser[]>("/api/users");
}

export function listInvites(): Promise<InviteRecord[]> {
  return request<InviteRecord[]>("/api/users/invites");
}

export function createInvite(role: UserRole): Promise<CreatedInvite> {
  return request<CreatedInvite>("/api/users/invite", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ role }),
  });
}

export function deleteUser(username: string): Promise<void> {
  return request<void>(`/api/users/${encodeURIComponent(username)}`, { method: "DELETE" });
}

export function setUserRole(username: string, role: UserRole): Promise<void> {
  return request<{ status: string }>(`/api/users/${encodeURIComponent(username)}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ role }),
  }).then(() => undefined);
}

export function getEngine(): Promise<EngineInfo> {
  return request<EngineInfo>("/api/engine");
}

export function getSwarm(): Promise<SwarmSnapshot> {
  return request<SwarmSnapshot>("/api/swarm");
}

export function getSystemInfo(): Promise<SystemInfo> {
  return request<SystemInfo>("/api/system/info");
}

export function getDiskUsage(): Promise<DiskUsage> {
  return request<DiskUsage>("/api/system/df");
}

export function getStorageInventory(): Promise<StorageInventory> {
  return request<StorageInventory>("/api/storage");
}

export function previewCleanup(plan: CleanupPlan): Promise<CleanupReport> {
  return request<CleanupReport>("/api/storage/cleanup/preview", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(plan),
  });
}

export function runCleanup(plan: CleanupPlan): Promise<CleanupReport> {
  return request<CleanupReport>("/api/storage/cleanup", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(plan),
  });
}

export function listContainers(): Promise<Container[]> {
  return request<Container[]>("/api/containers");
}

/** One stats sample for every running container, in a single round trip. */
export function getFleetStats(): Promise<ContainerStats[]> {
  return request<ContainerStats[]>("/api/stats");
}

export function runContainerAction(
  id: string,
  action: ContainerAction,
): Promise<void> {
  return request<{ status: string; id: string }>(
    `/api/containers/${encodeURIComponent(id)}/${action}`,
    { method: "POST" },
  ).then(() => undefined);
}

export interface LogQuery {
  tail?: string;
  timestamps?: boolean;
  since?: string;
}

export function logsUrl(id: string, opts: LogQuery = {}): string {
  const q = new URLSearchParams();
  q.set("tail", opts.tail ?? "200");
  if (opts.timestamps) q.set("timestamps", "1");
  if (opts.since) q.set("since", opts.since);
  return `/api/containers/${encodeURIComponent(id)}/logs?${q}`;
}

export function logsDownloadUrl(id: string, opts: LogQuery = {}): string {
  const q = new URLSearchParams();
  q.set("tail", opts.tail ?? "all");
  q.set("follow", "0");
  q.set("download", "1");
  if (opts.timestamps) q.set("timestamps", "1");
  if (opts.since) q.set("since", opts.since);
  return `/api/containers/${encodeURIComponent(id)}/logs?${q}`;
}

export function getContainerStats(id: string): Promise<ContainerStats> {
  return request<ContainerStats>(`/api/containers/${encodeURIComponent(id)}/stats`);
}

export function getContainerFilesystem(id: string): Promise<ContainerFS> {
  return request<ContainerFS>(`/api/containers/${encodeURIComponent(id)}/filesystem`);
}

export function listContainerFiles(id: string, path = "/"): Promise<DirListing> {
  const q = new URLSearchParams({ path });
  return request<DirListing>(`/api/containers/${encodeURIComponent(id)}/files?${q}`);
}

export function statContainerFile(id: string, path: string): Promise<FSEntry> {
  const q = new URLSearchParams({ path });
  return request<FSEntry>(`/api/containers/${encodeURIComponent(id)}/files/stat?${q}`);
}

export function getContainerChanges(id: string): Promise<FSChangeList> {
  return request<FSChangeList>(`/api/containers/${encodeURIComponent(id)}/changes`);
}

export async function downloadBlob(url: string, filename: string): Promise<void> {
  const res = await fetch(url, { credentials: "include" });
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new ApiError(res.status, body?.error ?? `download failed: ${res.status}`);
  }
  const blob = await res.blob();
  const href = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = href;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(href);
}

export function downloadContainerFile(id: string, path: string, filename: string): Promise<void> {
  const q = new URLSearchParams({ path });
  return downloadBlob(`/api/containers/${encodeURIComponent(id)}/files/content?${q}`, filename);
}

export interface CreateContainerPort {
  hostPort: string;
  containerPort: string;
  protocol: "tcp" | "udp";
}

export interface CreateContainerRequest {
  image: string;
  name: string;
  env: string[];
  ports: CreateContainerPort[];
  binds: string[];
  restartPolicy: "no" | "always" | "on-failure" | "unless-stopped";
  command?: string;
  network?: string;
  memoryBytes?: number;
}

export function createContainer(req: CreateContainerRequest): Promise<{ id: string }> {
  return request<{ status: string; id: string }>("/api/containers", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
}

export function removeContainer(id: string, force = false): Promise<void> {
  return request<{ status: string }>(
    `/api/containers/${encodeURIComponent(id)}?force=${force}`,
    { method: "DELETE" },
  ).then(() => undefined);
}

export function inspectContainer(id: string): Promise<unknown> {
  return request<unknown>(`/api/containers/${encodeURIComponent(id)}/inspect`);
}

// --- Images ---------------------------------------------------------------

export function listImages(): Promise<DockerImage[]> {
  return request<DockerImage[]>("/api/images");
}

export function removeImage(id: string, force = false): Promise<void> {
  return request<{ status: string }>(
    `/api/images?id=${encodeURIComponent(id)}&force=${force}`,
    { method: "DELETE" },
  ).then(() => undefined);
}

export function pruneImages(): Promise<PruneResult> {
  return request<PruneResult>("/api/images/prune", { method: "POST" });
}

/**
 * Pulls an image, invoking `onProgress` for each raw status line the daemon
 * emits. Returns once the stream ends (success or the connection drops);
 * throws if the initial request itself fails (bad reference, unauthorized).
 */
export function pullImage(
  reference: string,
  onProgress: (line: string) => void,
): Promise<void> {
  return fetch("/api/images/pull", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ reference }),
  }).then(async (res) => {
    if (!res.ok || !res.body) {
      const body = (await res.json().catch(() => null)) as { error?: string } | null;
      throw new ApiError(res.status, body?.error ?? `pull failed: ${res.status}`);
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    for (;;) {
      const { done, value } = await reader.read();
      if (done) return;
      buffer += decoder.decode(value, { stream: true });
      const frames = buffer.split("\n\n");
      buffer = frames.pop() ?? "";
      for (const frame of frames) {
        const line = frame.replace(/^data: /, "").trim();
        if (line) onProgress(line);
      }
    }
  });
}

// --- Volumes ----------------------------------------------------------------

export function listVolumes(): Promise<DockerVolume[]> {
  return request<DockerVolume[]>("/api/volumes");
}

export function createVolume(name: string, driver: string): Promise<DockerVolume> {
  return request<DockerVolume>("/api/volumes", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, driver }),
  });
}

export function removeVolume(name: string, force = false): Promise<void> {
  return request<{ status: string }>(
    `/api/volumes?name=${encodeURIComponent(name)}&force=${force}`,
    { method: "DELETE" },
  ).then(() => undefined);
}

export function pruneVolumes(): Promise<PruneResult> {
  return request<PruneResult>("/api/volumes/prune", { method: "POST" });
}

// --- Networks -----------------------------------------------------------

export function listNetworks(): Promise<DockerNetwork[]> {
  return request<DockerNetwork[]>("/api/networks");
}

export function createNetwork(name: string, driver: string): Promise<DockerNetwork> {
  return request<DockerNetwork>("/api/networks", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, driver }),
  });
}

export function removeNetwork(id: string): Promise<void> {
  return request<{ status: string }>(`/api/networks?id=${encodeURIComponent(id)}`, {
    method: "DELETE",
  }).then(() => undefined);
}

export function pruneNetworks(): Promise<PruneResult> {
  return request<PruneResult>("/api/networks/prune", { method: "POST" });
}

export function listEnvironments(): Promise<Environment[]> {
  return request<Environment[]>("/api/environments");
}

export function createEnvironment(body: {
  name: string;
  kind: string;
  host: string;
  tlsCa: string;
  tlsCert: string;
  tlsKey: string;
}): Promise<Environment> {
  return request<Environment>("/api/environments", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function deleteEnvironment(id: string): Promise<void> {
  return request<void>(`/api/environments/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export function selectEnvironment(id: string): Promise<void> {
  return request<{ status: string }>(`/api/environments/${encodeURIComponent(id)}/select`, {
    method: "POST",
  }).then(() => undefined);
}

export function listStacks(): Promise<Stack[]> {
  return request<Stack[]>("/api/stacks");
}

export function getStack(id: string): Promise<Stack> {
  return request<Stack>(`/api/stacks/${encodeURIComponent(id)}`);
}

export function createStack(name: string, yaml: string): Promise<Stack> {
  return request<Stack>("/api/stacks", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, yaml }),
  });
}

export function createStackFromGit(body: {
  name: string;
  gitUrl: string;
  gitRef?: string;
  composeFile?: string;
  gitUsername?: string;
  gitToken?: string;
}): Promise<Stack> {
  return request<Stack>("/api/stacks", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function syncStack(id: string): Promise<Stack> {
  return request<Stack>(`/api/stacks/${encodeURIComponent(id)}/sync`, { method: "POST" });
}

export function updateStack(id: string, yaml: string): Promise<Stack> {
  return request<Stack>(`/api/stacks/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ yaml }),
  });
}

export function deleteStack(id: string): Promise<void> {
  return request<void>(`/api/stacks/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export function upStack(id: string): Promise<void> {
  return request<{ status: string }>(`/api/stacks/${encodeURIComponent(id)}/up`, { method: "POST" }).then(
    () => undefined,
  );
}

export function downStack(id: string, volumes = false): Promise<void> {
  return request<{ status: string }>(
    `/api/stacks/${encodeURIComponent(id)}/down?volumes=${volumes}`,
    { method: "POST" },
  ).then(() => undefined);
}

export function listRegistries(): Promise<Registry[]> {
  return request<Registry[]>("/api/registries");
}

export function upsertRegistry(host: string, username: string, password: string): Promise<Registry> {
  return request<Registry>("/api/registries", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ host, username, password }),
  });
}

export function deleteRegistry(id: string): Promise<void> {
  return request<void>(`/api/registries/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export function getImageHistory(id: string): Promise<ImageLayer[]> {
  return request<ImageLayer[]>(`/api/images/history?id=${encodeURIComponent(id)}`);
}

export function listVolumeFiles(name: string, path = "/"): Promise<DirListing> {
  return request<DirListing>(
    `/api/volumes/${encodeURIComponent(name)}/files?path=${encodeURIComponent(path)}`,
  );
}

export function volumeFileContentUrl(name: string, path: string): string {
  return `/api/volumes/${encodeURIComponent(name)}/files/content?path=${encodeURIComponent(path)}`;
}
