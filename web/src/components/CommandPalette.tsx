import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Box, CornerDownLeft, LogOut, Moon, Search, Settings, Sun } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useContainers } from "../hooks/useContainers";
import { useTheme } from "../hooks/useTheme";
import { NAV_SECTIONS } from "../nav";
import type { ContainerState } from "../types/domain";

interface PaletteItem {
  id: string;
  group: "Go to" | "Containers" | "Commands";
  label: string;
  hint?: string;
  icon: LucideIcon;
  state?: ContainerState;
  run: (opts: { alt: boolean }) => void;
}

/**
 * Subsequence match with a bonus for contiguous runs and word starts, so
 * "ngx" finds "nginx-proxy" and "api" ranks "api-gateway" above "rapid".
 * Returns null when the query is not a subsequence of the text.
 */
function score(text: string, query: string): number | null {
  if (!query) return 0;
  const t = text.toLowerCase();
  const q = query.toLowerCase();
  let ti = 0;
  let total = 0;
  let streak = 0;
  for (const ch of q) {
    const found = t.indexOf(ch, ti);
    if (found === -1) return null;
    streak = found === ti ? streak + 1 : 0;
    const wordStart = found === 0 || /[\s\-_/.:]/.test(t.charAt(found - 1));
    total += 1 + streak * 2 + (wordStart ? 3 : 0) - Math.min(found - ti, 5) * 0.2;
    ti = found + 1;
  }
  return total;
}

const STATE_DOT: Partial<Record<ContainerState, string>> = {
  running: "bg-ok",
  paused: "bg-warn",
  restarting: "bg-warn",
  dead: "bg-bad",
};

export function CommandPalette({ onClose, onLogout }: { onClose: () => void; onLogout: () => void }) {
  const navigate = useNavigate();
  const { theme, toggle } = useTheme();
  const { data: containers } = useContainers();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  const items = useMemo<PaletteItem[]>(() => {
    const go = (path: string) => () => {
      navigate(path);
      onClose();
    };
    const nav: PaletteItem[] = [
      ...NAV_SECTIONS.flatMap((s) => s.items).map((item) => ({
        id: `nav:${item.path}`,
        group: "Go to" as const,
        label: item.label,
        icon: item.icon,
        run: go(item.path),
      })),
      { id: "nav:/settings", group: "Go to", label: "Settings", icon: Settings, run: go("/settings") },
    ];
    const ctr: PaletteItem[] = (containers ?? []).map((c) => ({
      id: `ctr:${c.id}`,
      group: "Containers",
      label: c.name,
      hint: c.image,
      icon: Box,
      state: c.state,
      run: ({ alt }) => {
        navigate(`/containers?focus=${encodeURIComponent(c.id)}${alt ? "&tab=logs" : ""}`);
        onClose();
      },
    }));
    const cmds: PaletteItem[] = [
      {
        id: "cmd:theme",
        group: "Commands",
        label: theme === "dark" ? "Switch to light theme" : "Switch to dark theme",
        icon: theme === "dark" ? Sun : Moon,
        run: () => {
          toggle();
          onClose();
        },
      },
      {
        id: "cmd:logout",
        group: "Commands",
        label: "Sign out",
        icon: LogOut,
        run: () => {
          onClose();
          onLogout();
        },
      },
    ];
    return [...nav, ...ctr, ...cmds];
  }, [containers, navigate, onClose, onLogout, theme, toggle]);

  const results = useMemo(() => {
    const q = query.trim();
    if (!q) {
      const running = items.filter((i) => i.group === "Containers" && i.state === "running").slice(0, 6);
      return [...items.filter((i) => i.group === "Go to"), ...running, ...items.filter((i) => i.group === "Commands")];
    }
    return items
      .map((item) => ({ item, s: score(`${item.label} ${item.hint ?? ""}`, q) }))
      .filter((r): r is { item: PaletteItem; s: number } => r.s !== null)
      .sort((a, b) => b.s - a.s)
      .slice(0, 30)
      .map((r) => r.item);
  }, [items, query]);

  useEffect(() => setActive(0), [query]);

  useEffect(() => {
    listRef.current?.querySelector<HTMLElement>(`[data-index="${active}"]`)?.scrollIntoView({ block: "nearest" });
  }, [active]);

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      onClose();
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((i) => Math.min(i + 1, results.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      results[active]?.run({ alt: e.shiftKey });
    }
  };

  let lastGroup: string | undefined;

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center px-4 pt-[12vh]" role="dialog" aria-modal="true" aria-label="Command palette">
      <div className="absolute inset-0 bg-black/40 backdrop-blur-[2px] animate-fade-in" onClick={onClose} />
      <div className="relative w-full max-w-xl overflow-hidden rounded-xl border border-edge-strong bg-panel shadow-2xl animate-rise-in">
        <div className="flex items-center gap-3 border-b border-edge px-4">
          <Search className="h-4 w-4 shrink-0 text-ink-faint" />
          <input
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onKeyDown}
            placeholder="Search pages, containers, commands…"
            className="h-12 w-full bg-transparent text-sm text-ink outline-none placeholder:text-ink-faint"
          />
          <kbd className="rounded border border-edge bg-panel-2 px-1.5 py-0.5 font-mono text-[10px] text-ink-faint">esc</kbd>
        </div>

        <div ref={listRef} className="max-h-[50vh] overflow-y-auto p-1.5">
          {results.length === 0 && <p className="px-3 py-8 text-center text-sm text-ink-faint">No matches for “{query}”.</p>}
          {results.map((item, index) => {
            const header = !query.trim() && item.group !== lastGroup ? item.group : null;
            lastGroup = item.group;
            const isActive = index === active;
            return (
              <div key={item.id}>
                {header && <p className="label-caps px-2.5 pb-1 pt-2.5">{header}</p>}
                <button
                  data-index={index}
                  onMouseMove={() => setActive(index)}
                  onClick={(e) => item.run({ alt: e.shiftKey })}
                  className={`flex w-full items-center gap-3 rounded-md px-2.5 py-2 text-left text-sm transition-colors ${
                    isActive ? "bg-accent/10 text-ink" : "text-ink-muted"
                  }`}
                >
                  <span className="relative">
                    <item.icon className={`h-4 w-4 ${isActive ? "text-accent" : "text-ink-faint"}`} />
                    {item.state && STATE_DOT[item.state] && (
                      <span className={`absolute -bottom-0.5 -right-0.5 h-1.5 w-1.5 rounded-full ring-2 ring-panel ${STATE_DOT[item.state]}`} />
                    )}
                  </span>
                  <span className="truncate font-medium">{item.label}</span>
                  {item.hint && <span className="truncate font-mono text-xs text-ink-faint">{item.hint}</span>}
                  <span className="ml-auto flex shrink-0 items-center gap-2">
                    {query.trim() && <span className="label-caps">{item.group}</span>}
                    {isActive && <CornerDownLeft className="h-3.5 w-3.5 text-ink-faint" />}
                  </span>
                </button>
              </div>
            );
          })}
        </div>

        <div className="flex items-center gap-4 border-t border-edge bg-panel-2 px-4 py-2 font-mono text-[10.5px] text-ink-faint">
          <span>↑↓ navigate</span>
          <span>↵ open</span>
          <span>⇧↵ container logs</span>
        </div>
      </div>
    </div>
  );
}
