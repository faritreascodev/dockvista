import { Check, Moon, Sun } from "lucide-react";
import { ACCENT_PRESETS, useAccentColor, type AccentPreset } from "../hooks/useAccentColor";
import { useTheme, type Theme } from "../hooks/useTheme";

export function SettingsPage() {
  const { theme, setTheme } = useTheme();
  const { accent, setAccent } = useAccentColor();

  return (
    <main className="flex-1 overflow-y-auto px-6 py-6">
      <h1 className="text-xl font-semibold text-ink">Settings</h1>
      <p className="mt-1 text-sm text-ink-muted">Personalize how DockVista looks.</p>

      <section className="mt-6 max-w-xl rounded-xl border border-edge bg-panel p-5">
        <h2 className="text-sm font-semibold text-ink">Appearance</h2>
        <p className="mt-1 text-xs text-ink-muted">Applies immediately and is remembered on this device.</p>

        <div className="mt-4">
          <p className="mb-2 text-xs font-medium text-ink-muted">Theme</p>
          <div className="flex gap-2">
            {(
              [
                { value: "light", label: "Light", Icon: Sun },
                { value: "dark", label: "Dark", Icon: Moon },
              ] as { value: Theme; label: string; Icon: typeof Sun }[]
            ).map(({ value, label, Icon }) => (
              <button
                key={value}
                onClick={() => setTheme(value)}
                className={`flex items-center gap-2 rounded-lg border px-4 py-2 text-sm font-medium transition ${
                  theme === value
                    ? "border-accent bg-accent/10 text-accent"
                    : "border-edge bg-panel-2 text-ink-muted hover:text-ink"
                }`}
              >
                <Icon className="h-4 w-4" />
                {label}
              </button>
            ))}
          </div>
        </div>

        <div className="mt-5">
          <p className="mb-2 text-xs font-medium text-ink-muted">Accent color</p>
          <div className="flex flex-wrap gap-2">
            {(Object.keys(ACCENT_PRESETS) as AccentPreset[]).map((preset) => (
              <button
                key={preset}
                onClick={() => setAccent(preset)}
                title={preset}
                aria-label={preset}
                className="flex h-8 w-8 items-center justify-center rounded-full ring-2 ring-offset-2 ring-offset-panel transition"
                style={{
                  backgroundColor: `rgb(${ACCENT_PRESETS[preset]})`,
                  // @ts-expect-error -- CSS custom property for the ring color
                  "--tw-ring-color": accent === preset ? `rgb(${ACCENT_PRESETS[preset]})` : "transparent",
                }}
              >
                {accent === preset && <Check className="h-4 w-4 text-white" />}
              </button>
            ))}
          </div>
        </div>
      </section>
    </main>
  );
}
