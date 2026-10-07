import { useEffect, useState, type ReactNode } from "react";
import { ArrowRight, FolderOpen, Info, Lock, ShieldAlert } from "lucide-react";
import { ApiError, getContainerFilesystem } from "../api/client";
import type { Container, ContainerFS, ContainerMount } from "../types/domain";
import { formatBytes, isDesktopHostPath } from "../utils/format";
import { Button } from "./ui/Button";
import { Chip } from "./ui/Table";

export function MountsTab({
  container,
  onOpenPath,
}: {
  container: Container;
  onOpenPath: (path: string) => void;
}) {
  const [fs, setFs] = useState<ContainerFS>();
  const [error, setError] = useState<string>();

  useEffect(() => {
    setFs(undefined);
    setError(undefined);
    getContainerFilesystem(container.id)
      .then(setFs)
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load mounts."));
  }, [container.id]);

  if (error) return <p className="px-4 py-6 text-sm text-bad">{error}</p>;
  if (!fs) return <p className="px-4 py-6 text-sm text-ink-muted">Loading…</p>;

  const binds = fs.mounts.filter((m) => m.type === "bind");
  const volumes = fs.mounts.filter((m) => m.type === "volume");
  const other = fs.mounts.filter((m) => m.type !== "bind" && m.type !== "volume");
  const wsl = binds.some((m) => isDesktopHostPath(m.source));

  return (
    <div className="space-y-3 px-4 py-3">
      {fs.privileged && (
        <Alert tone="bad" icon={<ShieldAlert className="h-3.5 w-3.5" />}>
          Privileged. This container can reach host devices the same way the daemon can.
        </Alert>
      )}
      {fs.readonlyRootfs && (
        <Alert tone="info" icon={<Lock className="h-3.5 w-3.5" />}>
          Read-only rootfs. Writes go to tmpfs or to the mounts below.
        </Alert>
      )}
      {wsl && (
        <Alert tone="info" icon={<Info className="h-3.5 w-3.5" />}>
          Bind sources under <span className="font-mono">/run/desktop</span> or <span className="font-mono">/host_mnt</span> are
          Windows paths inside Docker Desktop&apos;s WSL disk — the same place Storage reclaim does not shrink C:.
        </Alert>
      )}
      {volumes.length > 0 && (
        <Alert tone="info" icon={<Info className="h-3.5 w-3.5" />}>
          Named volumes survive container delete and image prune. Remove them from Storage, or label{" "}
          <span className="font-mono">dockvista.protect=true</span> if they must never be offered.
        </Alert>
      )}

      {fs.sizeKnown && (
        <p className="font-mono text-[11px] text-ink-faint">
          Writable layer {formatBytes(fs.sizeRw)}
          {fs.sizeRootFs > 0 && <> · image {formatBytes(fs.sizeRootFs)}</>}
        </p>
      )}

      {fs.mounts.length === 0 && <p className="text-sm text-ink-muted">No mounts. The container only has its image layers.</p>}

      {binds.length > 0 && (
        <section>
          <p className="label-caps mb-2">Binds · {binds.length}</p>
          <div className="space-y-2">{binds.map((m) => <MountCard key={m.destination} mount={m} onOpenPath={onOpenPath} />)}</div>
        </section>
      )}
      {volumes.length > 0 && (
        <section>
          <p className="label-caps mb-2">Volumes · {volumes.length}</p>
          <div className="space-y-2">{volumes.map((m) => <MountCard key={m.destination} mount={m} onOpenPath={onOpenPath} />)}</div>
        </section>
      )}
      {other.length > 0 && (
        <section>
          <p className="label-caps mb-2">Other · {other.length}</p>
          <div className="space-y-2">{other.map((m) => <MountCard key={m.destination} mount={m} onOpenPath={onOpenPath} />)}</div>
        </section>
      )}
    </div>
  );
}

function MountCard({ mount, onOpenPath }: { mount: ContainerMount; onOpenPath: (path: string) => void }) {
  return (
    <div className="rounded-lg border border-edge bg-panel-2 p-3">
      <div className="flex flex-wrap items-center gap-1.5">
        <Chip tone="accent">{mount.type}</Chip>
        <Chip tone={mount.rw ? "ok" : "warn"}>{mount.rw ? "rw" : "ro"}</Chip>
        {mount.name && <Chip>{mount.name}</Chip>}
        {mount.driver && mount.driver !== "local" && <Chip>{mount.driver}</Chip>}
      </div>
      <p className="mt-2 break-all font-mono text-[11px] text-ink-muted">{mount.source || "tmpfs"}</p>
      <p className="mt-1 flex items-start gap-1 break-all font-mono text-xs text-ink">
        <ArrowRight className="mt-0.5 h-3 w-3 shrink-0 text-ink-faint" />
        {mount.destination}
      </p>
      <Button size="sm" variant="ghost" className="mt-2" onClick={() => onOpenPath(mount.destination)}>
        <FolderOpen className="h-3.5 w-3.5" />
        Open in Files
      </Button>
    </div>
  );
}

function Alert({
  children,
  tone,
  icon,
}: {
  children: ReactNode;
  tone: "info" | "bad";
  icon: ReactNode;
}) {
  const cls = tone === "bad" ? "border-bad/30 bg-bad/10 text-bad" : "border-edge bg-panel-2 text-ink-muted";
  return (
    <p className={`flex items-start gap-2 rounded-md border px-3 py-2 text-[11px] leading-snug ${cls}`}>
      <span className="mt-0.5 shrink-0">{icon}</span>
      <span>{children}</span>
    </p>
  );
}
