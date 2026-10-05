import { useState } from "react";
import { Container, Moon, Sun } from "lucide-react";
import { ApiError } from "../api/client";
import { useTheme } from "../hooks/useTheme";

interface AuthScreenProps {
  mode: "setup" | "login";
  onSubmit: (username: string, password: string, setupToken?: string) => Promise<void>;
}

export function AuthScreen({ mode, onSubmit }: AuthScreenProps) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [setupToken, setSetupToken] = useState("");
  const [error, setError] = useState<string>();
  const [submitting, setSubmitting] = useState(false);
  const { theme, toggle } = useTheme();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(undefined);
    setSubmitting(true);
    try {
      await onSubmit(username, password, mode === "setup" ? setupToken : undefined);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Something went wrong. Try again.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="relative flex h-screen items-center justify-center bg-canvas text-ink">
      <button
        onClick={toggle}
        aria-label="Toggle theme"
        className="absolute right-4 top-4 rounded-lg border border-edge bg-panel p-2 text-ink-muted transition hover:text-ink"
      >
        {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
      </button>

      <div className="w-full max-w-sm rounded-xl border border-edge bg-panel p-8 shadow-xl">
        <div className="mb-6 text-center">
          <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-accent/10 text-accent">
            <Container className="h-5 w-5" />
          </div>
          <h1 className="text-lg font-semibold text-ink">DockVista</h1>
          <p className="mt-1 text-sm text-ink-muted">
            {mode === "setup" ? "Create the admin account to get started." : "Sign in to continue."}
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {mode === "setup" && (
            <div>
              <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="setup-token">
                Setup token
              </label>
              <input
                id="setup-token"
                type="text"
                autoComplete="off"
                required
                value={setupToken}
                onChange={(e) => setSetupToken(e.target.value)}
                className="w-full rounded-lg border border-edge bg-panel-2 px-3 py-2 text-sm text-ink outline-none focus:border-accent"
              />
              <p className="mt-1 text-xs text-ink-faint">Printed once in the server log on first launch.</p>
            </div>
          )}
          <div>
            <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="username">
              Username
            </label>
            <input
              id="username"
              type="text"
              autoComplete="username"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full rounded-lg border border-edge bg-panel-2 px-3 py-2 text-sm text-ink outline-none focus:border-accent"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="password">
              Password
            </label>
            <input
              id="password"
              type="password"
              autoComplete={mode === "setup" ? "new-password" : "current-password"}
              required
              minLength={mode === "setup" ? 8 : undefined}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full rounded-lg border border-edge bg-panel-2 px-3 py-2 text-sm text-ink outline-none focus:border-accent"
            />
            {mode === "setup" && <p className="mt-1 text-xs text-ink-faint">At least 8 characters.</p>}
          </div>

          {error && (
            <div className="rounded-lg border border-rose-300 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-300">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="w-full rounded-lg bg-accent px-3 py-2 text-sm font-medium text-accent-ink transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {submitting ? "Please wait…" : mode === "setup" ? "Create account" : "Sign in"}
          </button>
        </form>
      </div>
    </div>
  );
}
