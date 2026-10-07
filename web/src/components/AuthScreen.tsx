import { useEffect, useState } from "react";
import { ArrowRight, Moon, Sun } from "lucide-react";
import { ApiError, peekInvite, type UserRole } from "../api/client";
import { useTheme } from "../hooks/useTheme";
import { Logo } from "./Logo";
import { FieldLabel, FormError } from "./ui/Form";

interface AuthScreenProps {
  mode: "setup" | "login" | "invite";
  inviteToken?: string;
  onSubmit: (username: string, password: string, setupToken?: string) => Promise<void>;
}

const READOUT = [
  ["engine", "attached via /var/run/docker.sock"],
  ["events", "streaming over SSE"],
  ["exec", "pty over websocket"],
  ["auth", "invite-only · bcrypt session"],
] as const;

export function AuthScreen({ mode, inviteToken, onSubmit }: AuthScreenProps) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [setupToken, setSetupToken] = useState("");
  const [error, setError] = useState<string>();
  const [submitting, setSubmitting] = useState(false);
  const [inviteRole, setInviteRole] = useState<UserRole>();
  const [inviteInvalid, setInviteInvalid] = useState(false);
  const { theme, toggle } = useTheme();

  useEffect(() => {
    if (mode !== "invite" || !inviteToken) return;
    let cancelled = false;
    void peekInvite(inviteToken)
      .then((peek) => {
        if (!cancelled) setInviteRole(peek.role);
      })
      .catch(() => {
        if (!cancelled) setInviteInvalid(true);
      });
    return () => {
      cancelled = true;
    };
  }, [mode, inviteToken]);

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
    <div className="grid min-h-screen bg-canvas text-ink lg:grid-cols-[1.1fr_1fr]">
      <aside className="grid-paper relative hidden overflow-hidden border-r border-edge lg:block">
        <div className="absolute inset-0 bg-gradient-to-br from-accent/5 via-canvas/40 to-canvas" />
        <div className="scan-line pointer-events-none absolute inset-x-0 top-0 h-40" />
        <div className="relative flex h-full flex-col justify-between p-12">
          <div className="flex items-center gap-3">
            <Logo className="h-9 w-9" />
            <span className="text-lg font-semibold tracking-tight">DockVista</span>
          </div>

          <div>
            <p className="label-caps text-accent">self-hosted docker console</p>
            <h2 className="mt-3 max-w-md text-4xl font-semibold leading-[1.1] tracking-tight">
              Your engine, in plain sight.
            </h2>
            <p className="mt-4 max-w-md text-ink-muted">
              One binary. Live stats, logs, shells, and disk usage for every container, without handing your socket
              to a SaaS.
            </p>

            <div className="mt-10 max-w-md overflow-hidden rounded-lg border border-edge bg-panel/80 backdrop-blur">
              <div className="flex items-center gap-1.5 border-b border-edge px-3 py-2">
                <span className="h-2 w-2 rounded-full bg-edge-strong" />
                <span className="h-2 w-2 rounded-full bg-edge-strong" />
                <span className="h-2 w-2 rounded-full bg-edge-strong" />
                <span className="ml-2 font-mono text-[10.5px] text-ink-faint">dockvista --status</span>
              </div>
              <dl className="auth-readout space-y-1.5 px-4 py-3 font-mono text-xs">
                {READOUT.map(([k, v]) => (
                  <div key={k} className="flex gap-3">
                    <dt className="w-14 shrink-0 text-accent">{k}</dt>
                    <dd className="text-ink-muted">{v}</dd>
                  </div>
                ))}
              </dl>
            </div>
          </div>

          <p className="font-mono text-[11px] text-ink-faint">MIT licensed · open source</p>
        </div>
      </aside>

      <main className="relative flex items-center justify-center px-6 py-12">
        <button
          onClick={toggle}
          aria-label="Toggle theme"
          className="absolute right-4 top-4 flex h-9 w-9 items-center justify-center rounded-md border border-edge bg-panel text-ink-muted transition hover:text-ink"
        >
          {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
        </button>

        <div className="w-full max-w-sm animate-rise-in">
          <div className="mb-8 lg:hidden">
            <Logo className="h-10 w-10" />
          </div>
          <p className="label-caps">
            {mode === "setup" ? "first run" : mode === "invite" ? "invite" : "welcome back"}
          </p>
          <h1 className="mt-2 text-2xl font-semibold tracking-tight">
            {mode === "setup"
              ? "Create the admin account"
              : mode === "invite"
                ? "Join this DockVista"
                : "Sign in to DockVista"}
          </h1>
          <p className="mt-1.5 text-sm text-ink-muted">
            {mode === "setup"
              ? "This instance has no users yet. The setup token is printed once in the server log."
              : mode === "invite"
                ? inviteInvalid
                  ? "This invite is invalid or has already been used."
                  : inviteRole
                    ? `You've been invited as ${inviteRole}. Pick a username and password.`
                    : "Checking your invite…"
                : "Use the credentials for this instance. There is no public register."}
          </p>

          {mode === "invite" && inviteInvalid ? null : (
          <form onSubmit={handleSubmit} className="mt-8 space-y-4">
            {mode === "setup" && (
              <div>
                <FieldLabel htmlFor="setup-token" hint="from the server log">
                  Setup token
                </FieldLabel>
                <input
                  id="setup-token"
                  type="text"
                  autoComplete="off"
                  spellCheck={false}
                  required
                  value={setupToken}
                  onChange={(e) => setSetupToken(e.target.value)}
                  className="field font-mono"
                />
              </div>
            )}
            <div>
              <FieldLabel htmlFor="username">Username</FieldLabel>
              <input
                id="username"
                type="text"
                autoComplete="username"
                autoFocus={mode !== "setup"}
                required
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="field"
              />
            </div>
            <div>
              <FieldLabel htmlFor="password" {...(mode !== "login" ? { hint: "8–72 characters" } : {})}>
                Password
              </FieldLabel>
              <input
                id="password"
                type="password"
                autoComplete={mode === "login" ? "current-password" : "new-password"}
                required
                {...(mode === "login" ? {} : { minLength: 8, maxLength: 72 })}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="field"
              />
            </div>

            {error && <FormError>{error}</FormError>}

            <button
              type="submit"
              disabled={submitting}
              className="group flex h-10 w-full items-center justify-center gap-2 rounded-md bg-accent text-sm font-medium text-accent-ink shadow-[inset_0_1px_0_rgb(255_255_255/0.18)] transition hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {submitting ? (
                <span className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
              ) : (
                <>
                  {mode === "setup" ? "Create account" : mode === "invite" ? "Create account" : "Sign in"}
                  <ArrowRight className="h-4 w-4 transition group-hover:translate-x-0.5" />
                </>
              )}
            </button>
          </form>
          )}
        </div>
      </main>
    </div>
  );
}
