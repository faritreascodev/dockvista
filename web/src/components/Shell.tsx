import { useEffect, useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { Command, LogOut, Menu, Moon, Search, Settings, ShieldAlert, Sun, X } from "lucide-react";
import { useEngineInfo } from "../hooks/useEngineInfo";
import { useEnvironments } from "../hooks/useEnvironments";
import { useFleetStats } from "../hooks/useFleetStats";
import { useSearch } from "../hooks/useSearch";
import { useSession } from "../hooks/useSession";
import { useTheme } from "../hooks/useTheme";
import { NAV_SECTIONS } from "../nav";
import { CommandPalette } from "./CommandPalette";
import { Logo } from "./Logo";

// Pages whose lists are filtered by the header search box.
const FILTERABLE = ["/containers", "/compose", "/images", "/volumes", "/networks", "/storage"];

interface ShellProps {
  onLogout: () => void;
}

/** App-wide layout: sidebar, slim header, and whichever page is routed in. */
export function Shell({ onLogout }: ShellProps) {
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const { pathname } = useLocation();
  const { setQuery } = useSearch();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPaletteOpen((open) => !open);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  // A filter typed on one page rarely makes sense on the next.
  useEffect(() => {
    setQuery("");
    setMobileNavOpen(false);
  }, [pathname, setQuery]);

  return (
    <div className="flex h-screen bg-canvas text-ink">
      <aside className="hidden w-60 shrink-0 border-r border-edge bg-panel md:flex">
        <Sidebar onLogout={onLogout} />
      </aside>

      {mobileNavOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div className="absolute inset-0 bg-black/50 animate-fade-in" onClick={() => setMobileNavOpen(false)} />
          <aside className="relative flex h-full w-64 border-r border-edge bg-panel animate-slide-in">
            <Sidebar onLogout={onLogout} />
            <button
              onClick={() => setMobileNavOpen(false)}
              aria-label="Close navigation"
              className="absolute right-2 top-3 rounded-md p-1.5 text-ink-muted hover:bg-panel-2"
            >
              <X className="h-4 w-4" />
            </button>
          </aside>
        </div>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <Header
          filterable={FILTERABLE.includes(pathname)}
          onOpenPalette={() => setPaletteOpen(true)}
          onOpenNav={() => setMobileNavOpen(true)}
        />
        <Outlet />
      </div>

      {paletteOpen && <CommandPalette onClose={() => setPaletteOpen(false)} onLogout={onLogout} />}
    </div>
  );
}

function Sidebar({ onLogout }: { onLogout: () => void }) {
  const { username, role, readOnly, instanceReadOnly } = useSession();
  const { statsById } = useFleetStats();
  const runningCount = Object.keys(statsById).length;

  return (
    <div className="flex min-h-0 w-full flex-col">
      <div className="flex h-14 items-center gap-2.5 border-b border-edge px-4">
        <Logo className="h-7 w-7" />
        <div className="leading-none">
          <p className="text-[15px] font-semibold tracking-tight text-ink">DockVista</p>
          <p className="mt-1 font-mono text-[10px] uppercase tracking-[0.14em] text-ink-faint">docker console</p>
        </div>
      </div>

      <nav className="flex-1 space-y-5 overflow-y-auto px-2.5 py-4">
        {NAV_SECTIONS.map((section) => (
          <div key={section.title}>
            <p className="label-caps px-2 pb-1.5">{section.title}</p>
            <div className="space-y-0.5">
              {section.items.map((item) => (
                <NavLink
                  key={item.path}
                  to={item.path}
                  className={({ isActive }) =>
                    `group relative flex items-center gap-2.5 rounded-md px-2 py-1.5 text-sm transition-colors ${
                      isActive
                        ? "bg-panel-2 font-medium text-ink before:absolute before:-left-2.5 before:top-1.5 before:h-[calc(100%-12px)] before:w-[3px] before:rounded-r before:bg-accent"
                        : "text-ink-muted hover:bg-panel-2 hover:text-ink"
                    }`
                  }
                >
                  {({ isActive }) => (
                    <>
                      <item.icon className={`h-4 w-4 ${isActive ? "text-accent" : "text-ink-faint group-hover:text-ink-muted"}`} />
                      <span className="flex-1">{item.label}</span>
                      {item.path === "/containers" && runningCount > 0 && (
                        <span className="rounded bg-ok/10 px-1.5 font-mono text-[10.5px] font-medium tabular-nums text-ok">
                          {runningCount}
                        </span>
                      )}
                    </>
                  )}
                </NavLink>
              ))}
            </div>
          </div>
        ))}
      </nav>

      <div className="space-y-3 border-t border-edge p-3">
        <EnginePanel />

        {instanceReadOnly && (
          <div className="flex items-start gap-2 rounded-md border border-warn/30 bg-warn/10 px-2.5 py-2 text-xs text-warn">
            <ShieldAlert className="mt-px h-3.5 w-3.5 shrink-0" />
            <span>Read-only instance. Changes are disabled.</span>
          </div>
        )}
        {readOnly && !instanceReadOnly && (
          <div className="flex items-start gap-2 rounded-md border border-warn/30 bg-warn/10 px-2.5 py-2 text-xs text-warn">
            <ShieldAlert className="mt-px h-3.5 w-3.5 shrink-0" />
            <span>Viewer role. You can look, not change.</span>
          </div>
        )}

        <div className="flex items-center gap-2.5">
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-accent/15 font-mono text-xs font-semibold uppercase text-accent">
            {username.slice(0, 2)}
          </span>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium text-ink">{username}</p>
            <p className="font-mono text-[10.5px] uppercase tracking-wider text-ink-faint">{role}</p>
          </div>
          <NavLink
            to="/settings"
            title="Settings"
            className={({ isActive }) =>
              `rounded-md p-1.5 transition ${isActive ? "bg-panel-2 text-accent" : "text-ink-faint hover:bg-panel-2 hover:text-ink"}`
            }
          >
            <Settings className="h-4 w-4" />
          </NavLink>
          <button
            onClick={onLogout}
            title="Sign out"
            className="rounded-md p-1.5 text-ink-faint transition hover:bg-panel-2 hover:text-bad"
          >
            <LogOut className="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
  );
}

function EnginePanel() {
  const { data: engine } = useEngineInfo();
  const { environments, active, select } = useEnvironments();
  const up = engine?.reachable ?? false;

  return (
    <div className="rounded-md border border-edge bg-panel-2 px-2.5 py-2">
      <label className="label-caps text-ink-faint">Environment</label>
      <select
        className="mt-1 h-8 w-full rounded-md border border-edge bg-panel px-2 text-xs text-ink outline-none focus:border-accent"
        value={active?.id ?? "local"}
        onChange={(e) => void select(e.target.value)}
      >
        {environments.map((env) => (
          <option key={env.id} value={env.id}>
            {env.name}
            {env.reachable ? "" : " (down)"}
          </option>
        ))}
      </select>
      <div className="mt-2 flex items-center gap-2">
        <span className={`h-1.5 w-1.5 rounded-full ${engine === undefined ? "bg-ink-faint" : up ? "bg-ok live-dot" : "bg-bad"}`} />
        <span className="text-xs font-medium text-ink">
          {engine === undefined ? "Connecting…" : up ? "Engine online" : "Engine unreachable"}
        </span>
      </div>
      {up && (
        <p className="mt-1 font-mono text-[10.5px] text-ink-faint">
          api {engine?.version} · {engine?.containers} containers
        </p>
      )}
    </div>
  );
}

function Header({
  filterable,
  onOpenPalette,
  onOpenNav,
}: {
  filterable: boolean;
  onOpenPalette: () => void;
  onOpenNav: () => void;
}) {
  const { query, setQuery } = useSearch();
  const { theme, toggle } = useTheme();
  const isMac = typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform);

  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b border-edge bg-canvas/80 px-4 backdrop-blur md:px-6">
      <button
        onClick={onOpenNav}
        aria-label="Open navigation"
        className="rounded-md p-1.5 text-ink-muted hover:bg-panel-2 md:hidden"
      >
        <Menu className="h-5 w-5" />
      </button>

      {filterable ? (
        <div className="relative w-full max-w-sm">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-ink-faint" />
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filter this page…"
            className="h-8 w-full rounded-md border border-edge bg-panel pl-8 pr-3 text-sm text-ink outline-none transition placeholder:text-ink-faint focus:border-accent"
          />
        </div>
      ) : (
        <div className="flex-1" />
      )}

      <div className="ml-auto flex items-center gap-1.5">
        <button
          onClick={onOpenPalette}
          className="flex h-8 items-center gap-2 rounded-md border border-edge bg-panel px-2.5 text-xs text-ink-muted transition hover:border-edge-strong hover:text-ink"
        >
          <Command className="h-3.5 w-3.5" />
          <span className="hidden sm:inline">Jump to…</span>
          <kbd className="rounded border border-edge bg-panel-2 px-1 font-mono text-[10px] text-ink-faint">
            {isMac ? "⌘K" : "Ctrl K"}
          </kbd>
        </button>
        <button
          onClick={toggle}
          aria-label="Toggle theme"
          className="flex h-8 w-8 items-center justify-center rounded-md text-ink-muted transition hover:bg-panel-2 hover:text-ink"
        >
          {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
        </button>
      </div>
    </header>
  );
}
