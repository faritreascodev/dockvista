import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react";
import { Check, Copy, KeyRound, Monitor, Moon, Sun, Trash2, UserPlus } from "lucide-react";
import {
  ApiError,
  changePassword,
  createEnvironment,
  createInvite,
  deleteEnvironment,
  deleteRegistry,
  deleteUser,
  listInvites,
  listRegistries,
  listUsers,
  setUserRole,
  upsertRegistry,
  type AccountUser,
  type CreatedInvite,
  type InviteRecord,
  type UserRole,
} from "../api/client";
import { useEnvironments } from "../hooks/useEnvironments";
import { PageHeader } from "../components/PageHeader";
import { Button } from "../components/ui/Button";
import { ConfirmDialog } from "../components/ui/ConfirmDialog";
import { FieldLabel, FormError } from "../components/ui/Form";
import { useToast } from "../components/ui/toastContext";
import { ACCENT_PRESETS, useAccentColor, type AccentPreset } from "../hooks/useAccentColor";
import { useSession } from "../hooks/useSession";
import { useTheme, type Theme } from "../hooks/useTheme";

const MIN_PASSWORD = 8;
const MAX_PASSWORD_BYTES = 72;

export function SettingsPage() {
  const { username, role, readOnly, instanceReadOnly } = useSession();

  return (
    <main className="flex-1 overflow-y-auto px-4 py-6 md:px-8">
      <PageHeader kicker="Console" title="Settings" description={`Signed in as ${username} (${role}).`} />

      <div className="mt-6 grid max-w-5xl gap-5 lg:grid-cols-2">
        <AppearanceSection />
        <PasswordSection />
        <Section title="Keyboard" kicker="shortcuts">
          <dl className="space-y-2.5 text-sm">
            {(
              [
                ["Command palette", "Ctrl / ⌘ + K"],
                ["Open container logs from palette", "Shift + Enter"],
                ["Close drawer or dialog", "Esc"],
              ] as const
            ).map(([label, keys]) => (
              <div key={label} className="flex items-center justify-between gap-4">
                <dt className="text-ink-muted">{label}</dt>
                <dd>
                  <kbd className="rounded border border-edge bg-panel-2 px-1.5 py-0.5 font-mono text-[11px] text-ink">{keys}</kbd>
                </dd>
              </div>
            ))}
          </dl>
        </Section>
        <Section title="Instance" kicker="server">
          <dl className="space-y-2.5 text-sm">
            <div className="flex items-center justify-between">
              <dt className="text-ink-muted">Your role</dt>
              <dd className="font-mono text-xs capitalize text-ink">{role}</dd>
            </div>
            <div className="flex items-center justify-between">
              <dt className="text-ink-muted">Mode</dt>
              <dd className={`font-mono text-xs ${readOnly ? "text-warn" : "text-ok"}`}>{readOnly ? "read-only" : "read-write"}</dd>
            </div>
            <div className="flex items-center justify-between">
              <dt className="text-ink-muted">Session</dt>
              <dd className="font-mono text-xs text-ink">httpOnly cookie · HMAC · 30m idle</dd>
            </div>
          </dl>
          <p className="mt-3 text-xs text-ink-faint">
            Mutating calls must come from this origin. Eight failed logins lock that user from this
            address for 15 minutes. Auth and destructive actions append to{" "}
            <span className="font-mono">audit.log</span> in the data dir.
          </p>
          {instanceReadOnly && (
            <p className="mt-3 text-xs text-ink-faint">
              Started with <span className="font-mono">DOCKVISTA_READ_ONLY=true</span>: every Docker-mutating API route
              and the terminal are refused server-side. Account actions still work.
            </p>
          )}
          {readOnly && !instanceReadOnly && (
            <p className="mt-3 text-xs text-ink-faint">
              Viewer cannot start, stop, prune, or open a shell. That is enforced on the API, not just hidden in the UI.
            </p>
          )}
        </Section>
      </div>

      {role === "admin" && (
        <div className="mt-5 grid max-w-5xl gap-5">
          <UsersSection currentUsername={username} />
          <EnvironmentsSection />
          <RegistriesSection />
        </div>
      )}
    </main>
  );
}

function Section({ title, kicker, children }: { title: string; kicker: string; children: ReactNode }) {
  return (
    <section className="rounded-lg border border-edge bg-panel">
      <div className="flex items-baseline gap-2.5 border-b border-edge px-5 py-3">
        <h2 className="text-sm font-semibold text-ink">{title}</h2>
        <span className="label-caps">{kicker}</span>
      </div>
      <div className="px-5 py-4">{children}</div>
    </section>
  );
}

function AppearanceSection() {
  const { theme, setTheme } = useTheme();
  const { accent, setAccent } = useAccentColor();

  const themes: { value: Theme; label: string; Icon: typeof Sun }[] = [
    { value: "light", label: "Paper", Icon: Sun },
    { value: "dark", label: "Graphite", Icon: Moon },
  ];

  return (
    <Section title="Appearance" kicker="this device">
      <p className="label-caps mb-2">Theme</p>
      <div className="grid grid-cols-2 gap-2">
        {themes.map(({ value, label, Icon }) => (
          <button
            key={value}
            onClick={() => setTheme(value)}
            aria-pressed={theme === value}
            className={`flex items-center gap-2.5 rounded-md border px-3 py-2.5 text-left text-sm transition ${
              theme === value ? "border-accent bg-accent/10 text-ink" : "border-edge bg-panel-2 text-ink-muted hover:text-ink"
            }`}
          >
            <Icon className={`h-4 w-4 ${theme === value ? "text-accent" : ""}`} />
            <span className="font-medium">{label}</span>
            {theme === value && <Check className="ml-auto h-3.5 w-3.5 text-accent" />}
          </button>
        ))}
      </div>

      <p className="label-caps mb-2 mt-5">Accent</p>
      <div className="flex flex-wrap gap-2">
        {(Object.keys(ACCENT_PRESETS) as AccentPreset[]).map((preset) => (
          <button
            key={preset}
            onClick={() => setAccent(preset)}
            aria-pressed={accent === preset}
            className={`flex items-center gap-2 rounded-md border px-2.5 py-1.5 font-mono text-xs capitalize transition ${
              accent === preset ? "border-edge-strong bg-panel-2 text-ink" : "border-edge text-ink-muted hover:text-ink"
            }`}
          >
            <span className="h-3 w-3 rounded-sm" style={{ backgroundColor: ACCENT_PRESETS[preset] }} />
            {preset}
          </button>
        ))}
      </div>
      <p className="mt-4 flex items-center gap-1.5 text-xs text-ink-faint">
        <Monitor className="h-3.5 w-3.5" /> Defaults follow your OS until you pick one.
      </p>
    </Section>
  );
}

function PasswordSection() {
  const toast = useToast();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  const nextBytes = new TextEncoder().encode(next).length;
  const validationError =
    next && next.length < MIN_PASSWORD
      ? `At least ${MIN_PASSWORD} characters.`
      : nextBytes > MAX_PASSWORD_BYTES
        ? `At most ${MAX_PASSWORD_BYTES} bytes.`
        : confirm && next !== confirm
          ? "Passwords don't match."
          : undefined;
  const canSubmit = current && next && confirm && !validationError && !busy;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setBusy(true);
    setError(undefined);
    try {
      await changePassword(current, next);
      setCurrent("");
      setNext("");
      setConfirm("");
      toast.push("success", "Password changed. Other sessions were signed out.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not change the password.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Section title="Password" kicker="security">
      <form onSubmit={handleSubmit} className="space-y-3">
          <div>
            <FieldLabel htmlFor="pw-current">Current password</FieldLabel>
            <input
              id="pw-current"
              type="password"
              autoComplete="current-password"
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
              className="field"
            />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <FieldLabel htmlFor="pw-next" hint={`${MIN_PASSWORD}–${MAX_PASSWORD_BYTES}`}>
                New password
              </FieldLabel>
              <input
                id="pw-next"
                type="password"
                autoComplete="new-password"
                value={next}
                onChange={(e) => setNext(e.target.value)}
                className="field"
              />
            </div>
            <div>
              <FieldLabel htmlFor="pw-confirm">Confirm</FieldLabel>
              <input
                id="pw-confirm"
                type="password"
                autoComplete="new-password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                className="field"
              />
            </div>
          </div>
          {validationError && <p className="text-xs text-warn">{validationError}</p>}
          {error && <FormError>{error}</FormError>}
          <div className="flex items-center justify-between gap-3 pt-1">
            <p className="text-xs text-ink-faint">Changing it signs out every other session.</p>
            <Button type="submit" variant="primary" loading={busy} disabled={!canSubmit}>
              <KeyRound className="h-4 w-4" /> Update
            </Button>
          </div>
        </form>
    </Section>
  );
}

const ROLES: UserRole[] = ["admin", "operator", "viewer"];

function UsersSection({ currentUsername }: { currentUsername: string }) {
  const toast = useToast();
  const [users, setUsers] = useState<AccountUser[]>([]);
  const [invites, setInvites] = useState<InviteRecord[]>([]);
  const [role, setRole] = useState<UserRole>("viewer");
  const [issued, setIssued] = useState<CreatedInvite>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [removeTarget, setRemoveTarget] = useState<string>();

  const refresh = useCallback(async () => {
    const [nextUsers, nextInvites] = await Promise.all([listUsers(), listInvites()]);
    setUsers(nextUsers);
    setInvites(nextInvites);
  }, []);

  useEffect(() => {
    void refresh().catch((err: unknown) => {
      setError(err instanceof ApiError ? err.message : "Could not load users.");
    });
  }, [refresh]);

  const handleInvite = async () => {
    setBusy(true);
    setError(undefined);
    try {
      const created = await createInvite(role);
      setIssued(created);
      toast.push("success", `Invite created for ${role}. Copy it now — it is shown once.`);
      await refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create invite.");
    } finally {
      setBusy(false);
    }
  };

  const copyLink = async (path: string) => {
    const url = `${window.location.origin}${path}`;
    await navigator.clipboard.writeText(url);
    toast.push("success", "Invite link copied.");
  };

  const pending = invites.filter((inv) => !inv.used && new Date(inv.expiresAt).getTime() > Date.now());

  return (
    <Section title="Users" kicker="invite only">
      <p className="text-sm text-ink-muted">
        No public register. Send a 72-hour invite. Viewer is read-only on the API. Operator can change containers.
        Only admin can invite or change roles.
      </p>

      <div className="mt-4 flex flex-wrap items-end gap-3">
        <div>
          <FieldLabel htmlFor="invite-role">Role</FieldLabel>
          <select
            id="invite-role"
            value={role}
            onChange={(e) => setRole(e.target.value as UserRole)}
            className="field w-40"
          >
            {ROLES.map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
          </select>
        </div>
        <Button variant="primary" onClick={() => void handleInvite()} loading={busy}>
          <UserPlus className="h-4 w-4" /> Create invite
        </Button>
      </div>

      {issued && (
        <div className="mt-4 rounded-md border border-accent/30 bg-accent/5 px-3 py-3">
          <p className="text-xs font-medium text-ink">Shown once. Anyone with this link can create a {issued.role} account.</p>
          <p className="mt-1 break-all font-mono text-[11px] text-ink-muted">{`${window.location.origin}${issued.path}`}</p>
          <Button size="sm" className="mt-2" onClick={() => void copyLink(issued.path)}>
            <Copy className="h-3.5 w-3.5" /> Copy link
          </Button>
        </div>
      )}

      {error && <FormError>{error}</FormError>}

      <ul className="mt-5 divide-y divide-edge">
        {users.map((user) => (
          <li key={user.username} className="flex flex-wrap items-center gap-3 py-2.5">
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-ink">{user.username}</p>
              {user.username === currentUsername && <p className="text-[11px] text-ink-faint">you</p>}
            </div>
            <select
              aria-label={`Role for ${user.username}`}
              value={user.role}
              disabled={user.username === currentUsername}
              onChange={(e) => {
                const next = e.target.value as UserRole;
                void setUserRole(user.username, next)
                  .then(() => {
                    toast.push("success", `${user.username} is now ${next}. Their other sessions were signed out.`);
                    return refresh();
                  })
                  .catch((err: unknown) => {
                    toast.push("error", err instanceof ApiError ? err.message : "Could not change role.");
                  });
              }}
              className="field w-32"
            >
              {ROLES.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
            {user.username !== currentUsername && (
              <Button size="sm" variant="ghost" onClick={() => setRemoveTarget(user.username)} title="Remove user">
                <Trash2 className="h-4 w-4" />
              </Button>
            )}
          </li>
        ))}
      </ul>

      {pending.length > 0 && (
        <div className="mt-5">
          <p className="label-caps mb-2">Pending invites</p>
          <ul className="space-y-1.5 text-sm text-ink-muted">
            {pending.map((inv) => (
              <li key={inv.id} className="flex justify-between gap-3 font-mono text-xs">
                <span>
                  {inv.role} · by {inv.createdBy}
                </span>
                <span className="text-ink-faint">expires {new Date(inv.expiresAt).toLocaleString()}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {removeTarget && (
        <ConfirmDialog
          title={`Remove ${removeTarget}?`}
          description="They lose access immediately. Existing containers are not touched."
          confirmLabel="Remove"
          onConfirm={async () => {
            await deleteUser(removeTarget);
            toast.push("success", `${removeTarget} was removed.`);
            await refresh();
          }}
          onClose={() => setRemoveTarget(undefined)}
        />
      )}
    </Section>
  );
}

function EnvironmentsSection() {
  const { environments, refresh, select } = useEnvironments();
  const toast = useToast();
  const [name, setName] = useState("");
  const [host, setHost] = useState("tcp://");
  const [tlsCa, setTlsCa] = useState("");
  const [tlsCert, setTlsCert] = useState("");
  const [tlsKey, setTlsKey] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  const add = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(undefined);
    try {
      await createEnvironment({ name, kind: "tcp", host, tlsCa, tlsCert, tlsKey });
      toast.push("success", `Added ${name}.`);
      setName("");
      setHost("tcp://");
      setTlsCa("");
      setTlsCert("");
      setTlsKey("");
      await refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not add environment.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Section title="Environments" kicker="daemons">
      <p className="mb-3 text-xs text-ink-faint">
        One environment is one Docker engine. Local is this machine. TCP+TLS is another daemon — not a cluster.
        SSH is not in this release.
      </p>
      <ul className="space-y-2 text-sm">
        {environments.map((env) => (
          <li key={env.id} className="flex items-center justify-between gap-3 rounded-md border border-edge px-3 py-2">
            <div>
              <p className="font-medium text-ink">
                {env.name} {env.active && <span className="text-xs text-accent">active</span>}
              </p>
              <p className="font-mono text-[11px] text-ink-faint">{env.host || env.kind}</p>
            </div>
            <div className="flex gap-2">
              {!env.active && (
                <Button size="sm" onClick={() => void select(env.id)}>
                  Switch
                </Button>
              )}
              {!env.local && (
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={async () => {
                    await deleteEnvironment(env.id);
                    await refresh();
                  }}
                >
                  Remove
                </Button>
              )}
            </div>
          </li>
        ))}
      </ul>
      <form className="mt-4 space-y-3" onSubmit={add}>
        <FieldLabel htmlFor="env-name">Name</FieldLabel>
        <input id="env-name" className="field" value={name} onChange={(e) => setName(e.target.value)} required />
        <FieldLabel htmlFor="env-host" hint="tcp://host:2376">
          Host
        </FieldLabel>
        <input id="env-host" className="field font-mono" value={host} onChange={(e) => setHost(e.target.value)} required />
        <FieldLabel htmlFor="env-ca">CA PEM</FieldLabel>
        <textarea id="env-ca" className="field font-mono" rows={3} value={tlsCa} onChange={(e) => setTlsCa(e.target.value)} required />
        <FieldLabel htmlFor="env-cert">Client cert PEM</FieldLabel>
        <textarea id="env-cert" className="field font-mono" rows={3} value={tlsCert} onChange={(e) => setTlsCert(e.target.value)} required />
        <FieldLabel htmlFor="env-key">Client key PEM</FieldLabel>
        <textarea id="env-key" className="field font-mono" rows={3} value={tlsKey} onChange={(e) => setTlsKey(e.target.value)} required />
        {error && <FormError>{error}</FormError>}
        <Button type="submit" variant="primary" loading={busy}>
          Add TCP environment
        </Button>
      </form>
    </Section>
  );
}

function RegistriesSection() {
  const toast = useToast();
  const [items, setItems] = useState<{ id: string; host: string; username: string }[]>([]);
  const [host, setHost] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  const refresh = async () => setItems(await listRegistries());
  useEffect(() => {
    void refresh();
  }, []);

  return (
    <Section title="Registries" kicker="private pull">
      <p className="mb-3 text-xs text-ink-faint">
        Stored in the data dir (0600). Used when pulling an image whose registry host matches.
      </p>
      <ul className="mb-3 space-y-2 text-sm">
        {items.map((it) => (
          <li key={it.id} className="flex items-center justify-between gap-3 rounded-md border border-edge px-3 py-2">
            <span className="font-mono text-xs">
              {it.username}@{it.host}
            </span>
            <Button
              size="sm"
              variant="ghost"
              onClick={async () => {
                await deleteRegistry(it.id);
                await refresh();
              }}
            >
              Remove
            </Button>
          </li>
        ))}
      </ul>
      <form
        className="grid gap-3 sm:grid-cols-3"
        onSubmit={async (e) => {
          e.preventDefault();
          await upsertRegistry(host, username, password);
          toast.push("success", `Saved credentials for ${host}.`);
          setPassword("");
          await refresh();
        }}
      >
        <input className="field font-mono" placeholder="ghcr.io" value={host} onChange={(e) => setHost(e.target.value)} required />
        <input className="field" placeholder="username" value={username} onChange={(e) => setUsername(e.target.value)} required />
        <input className="field" type="password" placeholder="token / password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        <div className="sm:col-span-3">
          <Button type="submit" variant="primary">
            Save registry
          </Button>
        </div>
      </form>
    </Section>
  );
}
