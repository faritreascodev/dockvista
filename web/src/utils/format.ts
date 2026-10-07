export function formatBytes(bytes: number): string {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const exponent = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1,
  );
  const value = bytes / 1024 ** exponent;
  return `${value >= 100 || exponent === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[exponent]}`;
}

export function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`;
}

export function formatBytesPerSec(bytesPerSec: number): string {
  return `${formatBytes(Math.max(0, bytesPerSec))}/s`;
}

/** Docker Desktop on Windows bind-mounts the host through WSL paths. */
export function isDesktopHostPath(source: string): boolean {
  const s = source.toLowerCase();
  return (
    s.includes("/run/desktop/") ||
    s.includes("/host_mnt/") ||
    s.includes("docker_data") ||
    /^[a-z]:[\\/]/.test(source)
  );
}

export function formatRelativeIso(iso?: string): string {
  if (!iso) return "—";
  const ms = Date.parse(iso);
  if (Number.isNaN(ms)) return "—";
  return formatRelativeTime(ms / 1000);
}

export function formatRelativeTime(unixSeconds: number): string {
  const deltaMs = Date.now() - unixSeconds * 1000;
  const deltaSec = Math.max(0, Math.round(deltaMs / 1000));

  const units: [string, number][] = [
    ["year", 31536000],
    ["month", 2592000],
    ["day", 86400],
    ["hour", 3600],
    ["minute", 60],
  ];

  for (const [label, secondsInUnit] of units) {
    const amount = Math.floor(deltaSec / secondsInUnit);
    if (amount >= 1) {
      return `${amount} ${label}${amount > 1 ? "s" : ""} ago`;
    }
  }
  return "just now";
}

export function shortId(id: string): string {
  return id.slice(0, 12);
}

export function shortImageId(id: string): string {
  return shortId(id.replace(/^sha256:/, ""));
}
