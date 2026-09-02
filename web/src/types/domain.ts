// Mirrors the JSON DTOs emitted by internal/adapters/httpapi/dto.go — keep
// these two in sync whenever the API response shape changes.

export type ContainerState =
  | "created"
  | "running"
  | "paused"
  | "restarting"
  | "removing"
  | "exited"
  | "dead";

export interface Port {
  privatePort: number;
  publicPort?: number;
  type: string;
}

export interface Container {
  id: string;
  name: string;
  image: string;
  state: ContainerState;
  status: string;
  createdAt: number; // unix seconds
  ports: Port[];
  labels?: Record<string, string>;
}

export interface ContainerStats {
  containerId: string;
  cpuPercent: number;
  memoryUsageBytes: number;
  memoryLimitBytes: number;
  memoryPercent: number;
}

export interface EngineInfo {
  version: string;
  reachable: boolean;
  containers: number;
}

export interface LogLine {
  stream: "stdout" | "stderr";
  message: string;
}

export type ContainerAction =
  | "start"
  | "stop"
  | "pause"
  | "unpause"
  | "restart";

export interface DockerEvent {
  type: string; // "container" | "image" | "volume" | "network" | ...
  action: string; // "start" | "die" | "create" | "pull" | ...
  actorId: string;
  attributes?: Record<string, string>;
  time: string; // ISO 8601
}

export interface DockerImage {
  id: string;
  repoTags: string[];
  repoDigests: string[];
  sizeBytes: number;
  containers: number; // -1 if unknown
  createdAt: number;
  labels?: Record<string, string>;
}

export interface DockerVolume {
  name: string;
  driver: string;
  mountpoint: string;
  createdAt: number;
  labels?: Record<string, string>;
}

export interface DockerNetwork {
  id: string;
  name: string;
  driver: string;
  scope: string;
  internal: boolean;
  attachable: boolean;
  createdAt: number;
  labels?: Record<string, string>;
  containers?: Record<string, string>;
}

export interface PruneResult {
  deleted: number;
  spaceReclaimedBytes?: number;
}
