import { useCallback, useEffect, useState } from "react";

export type Theme = "light" | "dark";

const STORAGE_KEY = "dockvista-theme";

function systemPrefersDark(): boolean {
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

function readStoredTheme(): Theme | null {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    return stored === "dark" || stored === "light" ? stored : null;
  } catch {
    return null;
  }
}

function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
}

/**
 * Reads/writes the light-dark preference, persisted to localStorage and
 * applied as a `.dark` class on <html> (see tailwind.config.js darkMode:
 * "class"). index.html applies the stored/system preference synchronously
 * before React mounts to avoid a flash of the wrong theme; this hook keeps
 * that in sync afterward.
 */
export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(
    () => readStoredTheme() ?? (systemPrefersDark() ? "dark" : "light"),
  );

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  const setTheme = useCallback((next: Theme) => {
    setThemeState(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      /* localStorage unavailable — theme still applies for this session. */
    }
  }, []);

  const toggle = useCallback(() => {
    setTheme(theme === "dark" ? "light" : "dark");
  }, [theme, setTheme]);

  return { theme, setTheme, toggle };
}
