import { useEffect, useState, type ReactNode } from "react";
import { ChevronRight, Download, File as FileIcon, Folder, HardDrive, Info } from "lucide-react";
import { ApiError, downloadContainerFile, getContainerChanges, getContainerFilesystem, listContainerFiles } from "../api/client";
import type { Container, ContainerFS, DirListing, FSChange, FSEntry } from "../types/domain";
import { formatBytes } from "../utils/format";
import { Button } from "./ui/Button";
import { Segmented } from "./ui/Segmented";
import { Chip } from "./ui/Table";
import { useToast } from "./ui/toastContext";

type Mode = "browse" | "changes";

function parentPath(p: string): string {
  if (p === "/") return "/";
  const i = p.lastIndexOf("/");
  return i <= 0 ? "/" : p.slice(0, i);
}

function crumbs(p: string): { label: string; path: string }[] {
  if (p === "/") return [{ label: "/", path: "/" }];
  const parts = p.split("/").filter(Boolean);
  const out = [{ label: "/", path: "/" }];
  let cur = "";
  for (const part of parts) {
    cur += `/${part}`;
    out.push({ label: part, path: cur });
  }
  return out;
}

export function FilesTab({
  container,
  jumpPath,
}: {
  container: Container;
  jumpPath?: string;
}) {
  const toast = useToast();
  const [mode, setMode] = useState<Mode>("browse");
  const [path, setPath] = useState(jumpPath || "/");
  const [fs, setFs] = useState<ContainerFS>();
  const [listing, setListing] = useState<DirListing>();
  const [changes, setChanges] = useState<{ added: number; modified: number; deleted: number; truncated?: boolean; changes: FSChange[] }>();
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<FSEntry>();
  const [downloading, setDownloading] = useState(false);

  useEffect(() => {
    if (jumpPath) setPath(jumpPath);
  }, [jumpPath]);

  useEffect(() => {
    setPath("/");
    setMode("browse");
    setSelected(undefined);
  }, [container.id]);

  useEffect(() => {
    let cancelled = false;
    getContainerFilesystem(container.id)
      .then((data) => {
        if (!cancelled) setFs(data);
      })
      .catch(() => {
        /* listing still works without the size summary */
      });
    return () => {
      cancelled = true;
    };
  }, [container.id]);

  useEffect(() => {
    if (mode !== "browse") return;
    let cancelled = false;
    setLoading(true);
    setError(undefined);
    setSelected(undefined);
    listContainerFiles(container.id, path)
      .then((data) => {
        if (cancelled) return;
        setListing(data);
        if (!data.dir) setSelected(data.entry);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof ApiError ? err.message : "Failed to list files.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [container.id, path, mode]);

  useEffect(() => {
    if (mode !== "changes") return;
    let cancelled = false;
    setLoading(true);
    setError(undefined);
    getContainerChanges(container.id)
      .then((data) => {
        if (!cancelled) setChanges(data);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof ApiError ? err.message : "Failed to load changes.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [container.id, mode]);

  async function download(entry: FSEntry) {
    if (entry.dir) return;
    setDownloading(true);
    try {
      await downloadContainerFile(container.id, entry.path, entry.name || "file");
    } catch (err) {
      toast.push("error", err instanceof ApiError ? err.message : "Download failed.");
    } finally {
      setDownloading(false);
    }
  }

  const reason = listing?.reason;
  const entries = listing?.entries ?? [];

  return (
    <div className="flex h-full flex-col">
      <div className="space-y-2 border-b border-edge px-4 py-2">
        <div className="flex items-center justify-between gap-2">
          <Segmented
            value={mode}
            onChange={setMode}
            options={[
              { value: "browse", label: "Browse" },
              changes
                ? { value: "changes", label: "Changes", count: changes.added + changes.modified + changes.deleted }
                : { value: "changes", label: "Changes" },
            ]}
          />
          {fs?.sizeKnown && (
            <p className="font-mono text-[11px] text-ink-faint">
              layer {formatBytes(fs.sizeRw)}
              {fs.sizeRootFs > 0 && <span> of {formatBytes(fs.sizeRootFs)}</span>}
            </p>
          )}
        </div>
        {mode === "browse" && (
          <nav className="flex min-w-0 flex-wrap items-center gap-0.5 font-mono text-[11px]" aria-label="Path">
            {crumbs(path).map((c, i) => (
              <span key={c.path} className="flex items-center">
                {i > 0 && <ChevronRight className="h-3 w-3 text-ink-faint" />}
                <button
                  type="button"
                  onClick={() => setPath(c.path)}
                  className={`rounded px-1 py-0.5 hover:bg-panel-2 hover:text-ink ${c.path === path ? "text-ink" : "text-ink-muted"}`}
                >
                  {c.label}
                </button>
              </span>
            ))}
          </nav>
        )}
      </div>

      {fs && (fs.sizeKnown || mode === "changes") && (
        <p className="flex items-start gap-2 border-b border-edge bg-panel-2/40 px-4 py-2 text-[11px] leading-snug text-ink-muted">
          <Info className="mt-0.5 h-3.5 w-3.5 shrink-0 text-info" />
          Changes is the writable layer versus the image — the same bytes Storage counts as this container. Bind mounts are not in that layer.
        </p>
      )}

      {error && <p className="px-4 py-3 text-sm text-bad">{error}</p>}
      {loading && !error && <p className="px-4 py-6 text-sm text-ink-muted">Loading…</p>}

      {mode === "browse" && !loading && listing && (
        <>
          {reason === "not_running" && (
            <Notice>
              Directory listing needs a running container with a shell. Changes and Bind mounts still work, and you can download a known file path.
            </Notice>
          )}
          {reason === "no_shell" && (
            <Notice>
              This image has no <span className="font-mono">ls</span> (distroless / scratch). Use Changes for the writable layer, or Bind mounts for host paths.
            </Notice>
          )}
          {listing.truncated && <Notice>Listing capped at 500 names.</Notice>}
          <ul className="min-h-0 flex-1 overflow-y-auto">
            {path !== "/" && (
              <li>
                <button
                  type="button"
                  onClick={() => setPath(parentPath(path))}
                  className="flex w-full items-center gap-2 px-4 py-1.5 text-left text-sm text-ink-muted hover:bg-panel-2"
                >
                  <Folder className="h-3.5 w-3.5" />
                  ..
                </button>
              </li>
            )}
            {entries.map((e) => (
              <li key={e.path}>
                <button
                  type="button"
                  onClick={() => (e.dir ? setPath(e.path) : setSelected(e))}
                  className={`flex w-full items-center gap-2 px-4 py-1.5 text-left text-sm hover:bg-panel-2 ${
                    selected?.path === e.path ? "bg-accent/[0.08]" : ""
                  }`}
                >
                  {e.dir ? (
                    <Folder className="h-3.5 w-3.5 shrink-0 text-accent" />
                  ) : (
                    <FileIcon className="h-3.5 w-3.5 shrink-0 text-ink-faint" />
                  )}
                  <span className="min-w-0 flex-1 truncate font-mono text-xs text-ink">{e.name}</span>
                  {e.mount && <Chip tone="accent">{e.mountType || "mount"}</Chip>}
                </button>
              </li>
            ))}
            {entries.length === 0 && !reason && listing.dir && (
              <li className="px-4 py-6 text-sm text-ink-faint">Empty directory.</li>
            )}
          </ul>
          {selected && !selected.dir && (
            <div className="border-t border-edge px-4 py-3">
              <p className="truncate font-mono text-xs text-ink">{selected.path}</p>
              <div className="mt-2 flex items-center gap-2">
                <Button size="sm" onClick={() => download(selected)} disabled={downloading} loading={downloading}>
                  <Download className="h-3.5 w-3.5" />
                  Download
                </Button>
                <span className="font-mono text-[11px] text-ink-faint">max 32 MB · read only</span>
              </div>
            </div>
          )}
        </>
      )}

      {mode === "changes" && !loading && changes && (
        <div className="min-h-0 flex-1 overflow-y-auto">
          <div className="flex gap-2 px-4 py-2">
            <Chip tone="ok">A {changes.added}</Chip>
            <Chip tone="warn">C {changes.modified}</Chip>
            <Chip>D {changes.deleted}</Chip>
          </div>
          {changes.truncated && <Notice>Diff capped at 2500 entries; totals still include the rest.</Notice>}
          {changes.changes.length === 0 && <p className="px-4 py-6 text-sm text-ink-faint">Writable layer is clean versus the image.</p>}
          <ul>
            {changes.changes.map((ch) => (
              <li key={`${ch.kind}:${ch.path}`} className="flex items-center gap-2 px-4 py-1 font-mono text-[11px]">
                <span
                  className={`w-4 font-semibold ${
                    ch.kind === "A" ? "text-ok" : ch.kind === "D" ? "text-bad" : "text-warn"
                  }`}
                >
                  {ch.kind}
                </span>
                <span className="min-w-0 flex-1 truncate text-ink">{ch.path}</span>
                {ch.kind !== "D" && (
                  <button
                    type="button"
                    className="text-ink-muted hover:text-ink"
                    title="Download this file"
                    onClick={() => download({ name: ch.path.split("/").pop() || "file", path: ch.path, dir: false, sizeBytes: 0 })}
                  >
                    <Download className="h-3 w-3" />
                  </button>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function Notice({ children }: { children: ReactNode }) {
  return (
    <p className="flex items-start gap-2 border-b border-edge px-4 py-2 text-[11px] leading-snug text-ink-muted">
      <HardDrive className="mt-0.5 h-3.5 w-3.5 shrink-0" />
      <span>{children}</span>
    </p>
  );
}
