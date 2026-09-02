import { useEffect, useState } from "react";

export const ACCENT_PRESETS = {
  sky: "14 165 233",
  violet: "139 92 246",
  emerald: "16 185 129",
  rose: "244 63 94",
  amber: "245 158 11",
  teal: "20 184 166",
} as const;

export type AccentPreset = keyof typeof ACCENT_PRESETS;

const STORAGE_KEY = "dockvista-accent";

function isPreset(value: string | null): value is AccentPreset {
  return !!value && value in ACCENT_PRESETS;
}

/**
 * Lets the user pick an accent color from a small preset list, applied as
 * the --color-accent CSS variable (see index.css) so every `accent-*`
 * Tailwind utility picks it up without a rebuild. index.html applies the
 * stored preset before React mounts, same FOUC-avoidance approach as
 * useTheme.
 */
export function useAccentColor() {
  const [accent, setAccentState] = useState<AccentPreset>(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      return isPreset(stored) ? stored : "sky";
    } catch {
      return "sky";
    }
  });

  useEffect(() => {
    document.documentElement.style.setProperty("--color-accent", ACCENT_PRESETS[accent]);
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
