import { useState } from "react";
import { NavLink } from "react-router-dom";
import {
  Boxes,
  ChevronDown,
  Container,
  HardDrive,
  Image as ImageIcon,
  Layers,
  LogOut,
  Moon,
  Network,
  Search,
  Settings,
  Sun,
} from "lucide-react";
import type { EngineInfo } from "../types/domain";
import { useSearch } from "../hooks/useSearch";
import { useTheme } from "../hooks/useTheme";

const NAV_ITEMS = [
  { label: "Containers", path: "/containers", icon: Boxes, active: true },
  { label: "Images", path: "/images", icon: ImageIcon, active: true },
  { label: "Volumes", path: "/volumes", icon: HardDrive, active: true },
  { label: "Networks", path: "/networks", icon: Network, active: true },
  { label: "Compose", path: "/compose", icon: Layers, active: true },
] as const;

interface TopBarProps {
  engine: EngineInfo | undefined;
  username: string;
  onLogout: () => void;
}

export function TopBar({ engine, username, onLogout }: TopBarProps) {
  const { query, setQuery } = useSearch();
  const { theme, toggle } = useTheme();
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <header className="border-b border-edge bg-panel">
      <div className="flex items-center gap-4 px-6 py-3">
        <div className="flex shrink-0 items-center gap-2.5">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-accent/15 text-accent">
            <Container className="h-5 w-5" />
          </div>
          <span className="text-lg font-semibold text-ink">DockVista</span>
        </div>

        <div className="relative mx-auto w-full max-w-md">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink-faint" />
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search..."
            className="w-full rounded-lg border border-edge bg-panel-2 py-1.5 pl-9 pr-3 text-sm text-ink outline-none placeholder:text-ink-faint focus:border-accent"
          />
        </div>

        <div className="flex shrink-0 items-center gap-3 text-sm">
          <span className="hidden items-center gap-1.5 sm:flex">
            <span className={`h-2 w-2 rounded-full ${engine?.reachable ? "bg-emerald-500" : "bg-rose-500"}`} />
            <span className="text-ink-muted">
              {engine?.reachable ? "Engine running" : "Engine unreachable"}
              {engine?.version && <span className="text-ink-faint"> &bull; {engine.version}</span>}
            </span>
          </span>

          <button
            onClick={toggle}
            aria-label="Toggle theme"
            className="rounded-lg p-1.5 text-ink-muted transition hover:bg-panel-2 hover:text-ink"
          >
            {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </button>

          <div className="relative">
            <button
              onClick={() => setMenuOpen((v) => !v)}
              className="flex items-center gap-1.5 rounded-lg p-1 text-ink-muted transition hover:bg-panel-2"
            >
              <span className="flex h-7 w-7 items-center justify-center rounded-full bg-accent/15 text-xs font-semibold text-accent">
                {username.slice(0, 2).toUpperCase()}
              </span>
              <ChevronDown className="h-3.5 w-3.5" />
            </button>

            {menuOpen && (
              <>
                <div className="fixed inset-0 z-10" onClick={() => setMenuOpen(false)} />
                <div className="absolute right-0 z-20 mt-2 w-48 rounded-lg border border-edge bg-panel py-1 shadow-xl">
                  <div className="border-b border-edge px-3 py-2 text-xs text-ink-muted">
                    Signed in as <span className="font-medium text-ink">{username}</span>
                  </div>
                  <NavLink
                    to="/settings"
                    onClick={() => setMenuOpen(false)}
                    className="flex items-center gap-2 px-3 py-2 text-sm text-ink hover:bg-panel-2"
                  >
                    <Settings className="h-4 w-4" /> Settings
                  </NavLink>
                  <button
                    onClick={() => {
                      setMenuOpen(false);
                      onLogout();
                    }}
                    className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-ink hover:bg-panel-2"
                  >
                    <LogOut className="h-4 w-4" /> Sign out
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      <nav className="flex gap-1 px-6">
        {NAV_ITEMS.map((item) =>
          item.active ? (
            <NavLink
              key={item.label}
              to={item.path}
              className={({ isActive }) =>
                `flex items-center gap-1.5 border-b-2 px-3 py-2.5 text-sm font-medium transition-colors ${
                  isActive
                    ? "border-accent text-accent"
                    : "border-transparent text-ink-muted hover:text-ink"
                }`
              }
            >
              <item.icon className="h-4 w-4" />
              {item.label}
            </NavLink>
          ) : null,
        )}
      </nav>
    </header>
  );
}
