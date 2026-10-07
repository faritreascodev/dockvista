# DockVista roadmap

North star: **Docker Desktop** for the day-to-day console of one engine, **Portainer** for the ceiling (environments, real stacks, Swarm, git). DockVista stays a **single-binary web app**. No public register. No Electron. The Windows tray is a small launcher next to that binary, not a second UI.

Invite-only roles, not a signup form: a Docker console is host-root.

## Differentiators to keep

These are the product, not polish. New work should strengthen them, not sand them down.

- **Cautious by default.** Storage never force-deletes a running workload. `dockvista.protect=true` is a hard skip. Destructive actions ask you to type `DELETE`. Files can be *read* (list, stat, download); we do not write or delete inside a container from the UI.
- **Honest about the disk.** Reclaimable bytes are the **engine's** disk. On Docker Desktop for Windows that is a WSL2 VHDX, so Explorer's C: does not move until the user compacts it. Bind-mount paths that live under `/run/desktop` or `/host_mnt` get the same explanation.
- **Writable-layer Changes.** `docker diff` (A/C/D) is a first-class tab, not a buried inspect field. That layer *is* the container size Storage reports. Distroless / stopped containers can still show Changes when a full directory listing needs a shell.
- **One binary, loopback, CSP.** No inline scripts, session cookie, setup-token burn. Reaching an authenticated DockVista equals reaching the socket — we say so instead of pretending a UI sandbox exists.
- **Read-only is enforced on the API.** `DOCKVISTA_READ_ONLY` is a process flag. Viewer is the same `403` per session. The UI only hides the buttons.

## Now (this slice)

Git into the stack workspace is shipped. Swarm inventory (nodes + services) is next; init/join/scale stay after that view is honest.

## Next

Do these in order. Each one changes the trust model or the surface area; do not skip Swarm because extras look smaller.

### 1. Users (invite-only RBAC) — shipped

- [x] Invite links, no public `/register`.
- [x] Roles: **Admin** / **Operator** / **Viewer**.
- [x] Viewer is read-only in the API, not just hidden buttons.
- [x] Operator: lifecycle and cleanup, cannot invite or change roles.
- [x] Admin: users; protect-label policy and environments stay later.
- [x] Replace “one account per instance” in `docs/SYSTEM.md`.

### 2. Security hardening — shipped

Needed before a second engine or git clone exists.

- [x] CSRF / Origin check on mutating routes (and websocket upgrades). SameSite=Strict is not enough behind some proxies.
- [x] Login lockout after 8 failures (15 minutes, per IP+username).
- [x] Audit log: `audit.log` under the data dir, plus slog, for auth and destructive HTTP.
- [x] Idle timeout 30 minutes (`DOCKVISTA_IDLE_TIMEOUT`), 5 live cookies per user. `DOCKVISTA_COOKIE_SECURE` already existed.
- [x] govulncheck allowlist stays an explicit ID list in CI — do not “fix” CVEs by hiding them.

### 3. Production — shipped

- [x] GHCR image on tag (`ghcr.io/<owner>/dockvista`, linux/amd64+arm64).
- [x] `GET /readyz` pings the daemon; `dockvista readyz` is the distroless HEALTHCHECK. `/healthz` stays liveness (process up).
- [x] TLS overlay: `docker compose -f docker-compose.yml -f deploy/compose.tls.yml up -d` (Caddy, `DOCKVISTA_COOKIE_SECURE`).
- [x] Windows **tray launcher** (`cmd/dockvista-tray`): start/stop sibling `dockvista.exe`, open the browser, no extra desktop UI.

### 4. Environments (Portainer's real gap) — shipped

- [x] Named endpoints: local socket + TCP+TLS. SSH later.
- [x] One environment = one daemon. Cookie `dockvista_env` selects it. Storage, Files, and exec follow the active endpoint.
- [x] Admin adds/removes TCP endpoints; TLS PEMs stay in the data dir.

### 5. Compose of truth, then git

Today Compose still groups containers by `com.docker.compose.project`.

- [x] Paste a `compose.yml` into a **workspace DockVista owns** (`data/stacks/`) and `up`/`down` through the Docker API. No compose CLI, no host homedir. Bind mounts must stay inside the stack directory. `build:` is refused.
- [x] Git → clone **https** remotes into that workspace (go-git in-process, no git binary, no SSH/`file://`). Sync fetches and hard-resets. Private repos use a token stored 0600 in the data dir.

### 6. Swarm

- [x] Read inventory: cluster state, nodes, and services. An engine that is not in Swarm mode shows that instead of an empty fake cluster.
- Init, join, scale, and swarm stacks. Overlay networks already list under Networks. A single Docker Desktop on Windows is the wrong place to debut mutating Swarm.

### 7. Extra Docker that is worth it

- [x] Image history / layers.
- [x] Volume browser — only through a **running** container that already mounts the volume (same archive rules as Files; no inspector container).
- [x] Create-container: command + memory limit on top of env/ports/mounts.
- [x] Private registry credentials for pull.
- Optional image CVE summary — after git/Swarm, not as a substitute.

## Intentionally later / not this product

- Public registration.
- Electron / Tauri wrap of the whole UI.
- Auto-compact of `docker_data.vhdx` (it requires quitting Docker Desktop).
- Arbitrary host filesystem access.
- “Fix C: from Storage” — wrong disk; the notice is the fix.

## How to use this file

When a slice lands, check it off here and keep [README.md](../README.md) Features in sync. Do not grow a second backlog.
