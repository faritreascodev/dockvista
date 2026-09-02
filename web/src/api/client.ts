import type {
  Container,
  ContainerAction,
  ContainerStats,
  DockerImage,
  DockerNetwork,
  DockerVolume,
  EngineInfo,
  PruneResult,
} from "../types/domain";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { credentials: "include", ...init });
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new ApiError(res.status, body?.error ?? `request failed: ${res.status}`);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}

export interface AuthStatus {
  initialized: boolean;
}

export interface AuthUser {
  username: string;
}

export function getAuthStatus(): Promise<AuthStatus> {
  return request<AuthStatus>("/api/auth/status");
}

export function setupAdmin(username: string, password: string): Promise<void> {
  return request<{ status: string }>("/api/auth/setup", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
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

export function getEngine(): Promise<EngineInfo> {
  return request<EngineInfo>("/api/engine");
}

export function listContainers(): Promise<Container[]> {
  return request<Container[]>("/api/containers");
}

export function getContainerStats(id: string): Promise<ContainerStats> {
  return request<ContainerStats>(`/api/containers/${encodeURIComponent(id)}/stats`);
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

export function logsUrl(id: string, tail = 200): string {
  return `/api/containers/${encodeURIComponent(id)}/logs?tail=${tail}`;
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
