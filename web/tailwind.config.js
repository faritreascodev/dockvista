/** @type {import('tailwindcss').Config} */
export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        // Semantic, theme-aware tokens — values come from CSS custom
        // properties defined per-theme in src/index.css (:root for light,
        // .dark for dark), so a single className works in both themes
        // instead of every usage needing a `dark:` variant.
        canvas: "rgb(var(--color-canvas) / <alpha-value>)",
        panel: "rgb(var(--color-panel) / <alpha-value>)",
        "panel-2": "rgb(var(--color-panel-2) / <alpha-value>)",
        edge: "rgb(var(--color-edge) / <alpha-value>)",
        ink: "rgb(var(--color-ink) / <alpha-value>)",
        "ink-muted": "rgb(var(--color-ink-muted) / <alpha-value>)",
        "ink-faint": "rgb(var(--color-ink-faint) / <alpha-value>)",
        accent: "rgb(var(--color-accent) / <alpha-value>)",
        "accent-ink": "rgb(var(--color-accent-ink) / <alpha-value>)",
        // Legacy dark-only palette — kept only for any lingering
        // `surface-*` usage during the redesign; new code should use the
        // semantic tokens above.
        surface: {
          950: "#0a0e14",
          900: "#0f1420",
          850: "#131a28",
          800: "#182030",
        },
      },
    },
  },
  plugins: [],
};
