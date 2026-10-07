// Applied before React mounts so the first paint already has the right theme
// and accent. It is a separate file (not inline in index.html) because the
// Content-Security-Policy only allows same-origin scripts. Keep in sync with
// src/hooks/useTheme.ts and src/hooks/useAccentColor.ts.
(function () {
  try {
    var root = document.documentElement;
    var stored = localStorage.getItem("dockvista-theme");
    var dark = stored ? stored === "dark" : window.matchMedia("(prefers-color-scheme: dark)").matches;
    root.classList.toggle("dark", dark);

    var accent = localStorage.getItem("dockvista-accent");
    if (accent === "cyan" || accent === "violet" || accent === "lime" || accent === "coral") {
      root.setAttribute("data-accent", accent);
    }
  } catch {
    /* localStorage unavailable (private mode, etc.): defaults apply. */
  }
})();
