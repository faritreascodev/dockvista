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
  netRxBytes: number;
  netTxBytes: number;
  blockReadBytes: number;
  blockWriteBytes: number;
  sampledAt: number;
}

export interface EngineInfo {
  version: string;
  reachable: boolean;
  containers: number;
}

export interface LogLine {
  stream: "stdout" | "stderr";
  message: string;
  timestamp?: string;
}

export interface ContainerMount {
  type: string;
  name?: string;
  source: string;
  destination: string;
  driver?: string;
  mode?: string;
  rw: boolean;
  propagation?: string;
}

export interface ContainerFS {
  running: boolean;
  privileged: boolean;
  readonlyRootfs: boolean;
  sizeRw: number;
  sizeRootFs: number;
  sizeKnown: boolean;
  mounts: ContainerMount[];
}

export interface FSEntry {
  name: string;
  path: string;
  dir: boolean;
  sizeBytes: number;
  mode?: string;
  modTime?: string;
  linkTarget?: string;
  mount?: boolean;
  mountType?: string;
}

export interface DirListing {
  path: string;
  running: boolean;
  dir: boolean;
  reason?: "not_running" | "no_shell" | string;
  truncated?: boolean;
  entry: FSEntry;
  entries: FSEntry[];
}

export interface FSChange {
  path: string;
  kind: "A" | "C" | "D" | string;
}

export interface FSChangeList {
  changes: FSChange[];
  truncated?: boolean;
  added: number;
  modified: number;
  deleted: number;
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

export interface Environment {
  id: string;
  name: string;
  kind: "local" | "tcp" | string;
  host: string;
  reachable: boolean;
  version?: string;
  active: boolean;
  local: boolean;
}

export interface Stack {
  id: string;
  name: string;
  yaml?: string;
  createdAt: number;
  updatedAt: number;
}

export interface Registry {
  id: string;
  host: string;
  username: string;
}

export interface ImageLayer {
  id: string;
  createdAt: number;
  createdBy: string;
  sizeBytes: number;
  tags?: string[];
  comment?: string;
}

export interface SystemInfo {
  name: string;
  serverVersion: string;
  operatingSystem: string;
  osType: string;
  architecture: string;
  kernelVersion: string;
  storageDriver: string;
  cpus: number;
  memoryBytes: number;
  containers: number;
  containersRunning: number;
  containersPaused: number;
  containersStopped: number;
  images: number;
}

export interface DiskUsageCategory {
  count: number;
  active: number;
  sizeBytes: number;
  reclaimableBytes: number;
}

export interface DiskUsage {
  images: DiskUsageCategory;
  containers: DiskUsageCategory;
  volumes: DiskUsageCategory;
  buildCache: DiskUsageCategory;
}

export type StorageKind = "container" | "image" | "volume" | "buildcache";

export interface StorageItem {
  kind: StorageKind;
  id: string;
  name: string;
  sizeBytes: number;
  sizeKnown: boolean;
  createdAt?: string;
  lastUsedAt?: string;
  state?: string;
  project?: string;
  tags: string[];
  detail?: string;
  usedBy: string[];
  labels?: Record<string, string>;
  shared?: boolean;
  inUse: boolean;
  eligible: boolean;
  reason?: string;
}

export interface StorageInventory {
  generatedAt: string;
  usage: DiskUsage;
  items: StorageItem[];
}

export interface CleanupPlan {
  containers: string[];
  images: string[];
  volumes: string[];
  buildCache: boolean;
}

export interface CleanupResult {
  kind: StorageKind;
  id: string;
  name: string;
  status: "removed" | "skipped" | "failed" | "planned";
  message?: string;
  freedBytes: number;
}

export interface CleanupReport {
  dryRun: boolean;
  results: CleanupResult[];
  freedBytes: number;
}
