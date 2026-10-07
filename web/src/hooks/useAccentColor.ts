import { useEffect, useState } from "react";

/**
 * Accent presets. The actual RGB values live in index.css under
 * [data-accent="..."] so each preset can differ per theme; the swatch here
 * is only what the picker renders. "signal" is the default and has no
 * attribute rule (it is the :root / .dark value).
 */
export const ACCENT_PRESETS = {
  signal: "#f5a623",
  cyan: "#22d3ee",
  violet: "#a78bfa",
  lime: "#a3e635",
  coral: "#fb7185",
} as const;

export type AccentPreset = keyof typeof ACCENT_PRESETS;

const STORAGE_KEY = "dockvista-accent";
const DEFAULT_ACCENT: AccentPreset = "signal";

function isPreset(value: string | null): value is AccentPreset {
  return !!value && Object.prototype.hasOwnProperty.call(ACCENT_PRESETS, value);
}

/**
 * Lets the user pick an accent preset, applied as <html data-accent>.
 * public/theme-init.js applies the stored preset before React mounts so
 * the first paint already uses it.
 */
export function useAccentColor() {
  const [accent, setAccentState] = useState<AccentPreset>(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      return isPreset(stored) ? stored : DEFAULT_ACCENT;
    } catch {
      return DEFAULT_ACCENT;
    }
  });

  useEffect(() => {
    const root = document.documentElement;
    if (accent === DEFAULT_ACCENT) root.removeAttribute("data-accent");
    else root.setAttribute("data-accent", accent);
  }, [accent]);

  const setAccent = (next: AccentPreset) => {
    setAccentState(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      /* localStorage unavailable — accent still applies for this session. */
    }
  };

  return { accent, setAccent };
}
