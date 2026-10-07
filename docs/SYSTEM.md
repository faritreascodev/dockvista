# DockVista system

DockVista is one process. It serves the web UI, talks to a Docker daemon, and stores accounts as JSON under the data dir. There is no database. Extra users arrive through a one-time invite, not a public register.

## Runtime

`cmd/dockvista` loads configuration from the environment, starts an **environment hub** (one Docker client, cache, collector, and event bridge per named endpoint), and wires the adapters:

1. The **local** environment uses `DOCKER_HOST` / the platform socket. Extra **TCP+TLS** endpoints are stored under the data dir (`environments.json` + PEM files). The session cookie `dockvista_env` selects which daemon the API talks to. `/readyz` always pings local.
2. Per environment: a container-list cache, a collector on `DOCKVISTA_POLL_INTERVAL`, and an event bridge onto SSE (`GET /api/events`).
3. Auth: credentials and the HMAC key live under `DOCKVISTA_DATA_DIR`.
4. Image, volume, and network services call the daemon directly. They are not cached.
5. HTTP on `DOCKVISTA_ADDR` (default `127.0.0.1:8080`). The same server embeds `web/dist` for every non-API route.

Shutdown on SIGINT/SIGTERM drains in-flight requests for `DOCKVISTA_SHUTDOWN_TIMEOUT`.

## Layout

| Path | Role |
| --- | --- |
| `cmd/dockvista` | Process entry, dependency wiring, event bridge. `dockvista readyz` is the distroless health probe |
| `cmd/dockvista-tray` | Windows-only helper: start/stop the sibling binary, open the browser |
| `internal/core/domain` | Types only. No Docker SDK, no `net/http` |
| `internal/core/ports` | Interfaces the services depend on |
| `internal/core/service` | Use cases. Tested with fakes |
| `internal/adapters/enginehub` | One live runtime per environment |
| `internal/adapters/envstore` | Named TCP endpoints + TLS PEMs |
| `internal/adapters/stackstore` | Compose files under `stacks/` |
| `internal/adapters/regstore` | Private registry usernames/passwords |
| `internal/adapters/docker` | The only package that imports the Docker SDK |
| `internal/adapters/store` | Mutex-guarded container cache. Lookup by full ID, name, or a unique prefix of at least 12 characters |
| `internal/adapters/authstore` | `credentials.json` and `session_secret` |
| `internal/adapters/broker` | Pub/sub. A slow subscriber drops events instead of blocking the rest |
| `internal/adapters/httpapi` | REST, SSE, WebSocket, validation, auth, rate limits |
| `internal/platform/embedweb` | Embeds the built frontend |
| `web` | React + TypeScript UI |

`internal/core` never imports an adapter. Handlers depend on small interfaces declared next to the handler, not on the concrete service type.

## HTTP

Public routes: `GET /healthz` (process up), `GET /readyz` (daemon Ping), `GET /api/auth/status`, `POST /api/auth/setup`, `POST /api/auth/login`, `GET /api/auth/invite`, `POST /api/auth/accept`. Probe paths are not request-logged. The image HEALTHCHECK and `dockvista readyz` hit `/readyz`.

Everything else under `/api/` requires the session cookie. New routes added under that mux are authenticated by default.

| Area | Behavior |
| --- | --- |
| Containers | List comes from the cache. Start, stop, pause, unpause, restart, remove, create, inspect, and stats hit the daemon. Create tries to start; if start fails the response still returns the new ID (`started: false`) and the cache is refreshed |
| Logs | `GET /api/containers/{id}/logs` is SSE by default (`follow=1`). `timestamps=1`, `since`, and `tail` are passed to the daemon. `format=plain` (or `download=1`) returns `text/plain` instead of SSE, capped at 50 MB |
| Files | `GET /api/containers/{id}/filesystem` is inspect (mounts, writable-layer size, privileged). `GET /api/containers/{id}/files?path=` lists a directory via a one-shot `ls` when the container is running and has a shell; stopped / distroless listings return `reason` instead of a fake tree. `GET .../files/stat` and `GET .../files/content` use the archive API (32 MB download cap). Nothing writes into the container |
| Changes | `GET /api/containers/{id}/changes` is `docker diff` (A/C/D), capped at 2500 entries |
| Terminal | `GET /api/containers/{id}/exec` upgrades to a WebSocket and runs `/bin/sh -c` that execs `bash` when present, `sh` otherwise. Same-origin is required. The server pings the socket so idle shells survive proxies |
| Fleet stats | `GET /api/stats` returns CPU, memory, net and block I/O for every running container in one response, fetched with bounded concurrency. The UI polls it every 3s and pauses while the tab is hidden. The open Stats tab also polls `GET /api/containers/{id}/stats` every 1s |
| System | `GET /api/system/info` is engine version, OS, CPUs, and memory. `GET /api/system/df` is `docker system df`, cached for 15s and invalidated by any route that frees or adds disk (remove, prune, pull) |
| Images, volumes, networks | List, create, remove, prune. Image pull streams daemon progress as SSE |
| Events | `GET /api/events` is one broker subscription per tab |
| Users | Admin: `GET /api/users`, `POST /api/users/invite`, `PATCH/DELETE /api/users/{username}`. Public: `GET /api/auth/invite`, `POST /api/auth/accept`. Viewer cannot mutate Docker; Operator cannot manage users |
| Environments | `GET /api/environments`, `POST /api/environments/{id}/select`. Admin: create/delete TCP+TLS. SSH is 400 |
| Stacks | Workspace compose under the data dir. Paste YAML or `POST` with `gitUrl` (https only). `POST /api/stacks/{id}/sync` fetches a git stack. `/up` and `/down` talk to the **active** environment |
| Registries | Admin: `GET/PUT /api/registries`. Used as `X-Registry-Auth` on pull |
| Image history | `GET /api/images/history?id=` |
| Volume files | `GET /api/volumes/{name}/files` via a running container that mounts the volume |

Mutating routes are rate-limited (30 requests per minute per client address). Login and setup are limited to 5 per minute. POST/PUT/PATCH/DELETE and websocket upgrades also require `Origin` (or `Referer`) to match `Host`; a missing or foreign origin is `403`. Logs, events, and exec share a cap of 32 open streams.

Eight failed logins for the same username from the same client address lock that pair for 15 minutes (`429`). A session goes idle after 30 minutes without a verified request (`DOCKVISTA_IDLE_TIMEOUT`, `0s` disables idle). One account may hold five live cookies; a sixth login drops the least-recently used one.

Auth events and successful mutating API calls append a JSON line to `<DOCKVISTA_DATA_DIR>/audit.log` (`0600`) and emit `slog` with `audit=true`. Passwords are never written. The file rotates to `audit.log.1` at 2 MB.

With `DOCKVISTA_READ_ONLY=true`, every mutating route and the exec WebSocket answer `403` before reaching the service layer. A **Viewer** session gets the same `403` even when the process is read-write. `GET /api/auth/me` reports `readOnly` (process flag or viewer) and `instanceReadOnly` (process flag only) so the UI can hide the controls; the server check is the boundary. `POST /api/auth/password` stays available.

Responses set a strict `Content-Security-Policy` (`default-src 'self'`, `script-src 'self'`, `object-src 'none'`, `frame-ancestors 'none'`), `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, and `Referrer-Policy: no-referrer`. The UI has no inline scripts; the theme bootstrap is a static file.

## Auth

There is no default account in the binary.

On first launch, if `DOCKVISTA_SETUP_TOKEN` is unset, the process generates a token and logs it once at WARN. `POST /api/auth/setup` accepts that token a single time, with a username of 3–32 characters (`[A-Za-z0-9_.-]`) and a password of 8–72 bytes (bcrypt ignores anything past 72, so longer input is rejected instead of silently truncated). Once setup succeeds the process clears the token from memory. The first account is always an **admin**. Extra users are created only by accepting an invite (`GET /api/auth/invite`, `POST /api/auth/accept`). There is no public register.

Passwords are stored bcrypt-hashed in `<DOCKVISTA_DATA_DIR>/credentials.json` (version 2: `{version, users, invites}`). A v1 single-user file is wrapped as the admin on first read. The directory is created mode `0700` and the file mode `0600` on Unix. Windows does not apply those modes.

Roles: **Admin** (invite, change roles, delete users), **Operator** (Docker lifecycle and cleanup, not users), **Viewer** (`403` on mutating routes and exec). Admin user routes live under `GET/POST /api/users` and `PATCH/DELETE /api/users/{username}`. Invites expire after 72 hours; the plaintext token is returned once and stored as SHA-256.

Sessions are HMAC-SHA256 cookies named `dockvista_session` (`HttpOnly`, `SameSite=Strict`, 8 hours absolute, 30 minutes idle). The token includes a generation counter stored with **that** account. `POST /api/auth/logout` increments the caller's counter, so every previously issued cookie for that user stops verifying. Other users stay signed in. `POST /api/auth/password` and a role change do the same for that user. Set `DOCKVISTA_COOKIE_SECURE=true` when a proxy terminates TLS in front of the process. The process itself does not serve TLS.

govulncheck in CI allowlists reachable findings by OSV ID in `.github/workflows/ci.yml`. Drop an ID when upstream ships a fix. Do not add IDs to hide a new issue.

## How a container change reaches the UI

1. The daemon emits an event.
2. The bridge publishes it to every SSE subscriber and, for list-changing actions (`create`, `start`, `stop`, `die`, `destroy`, `pause`, `unpause`, `restart`, `rename`, `kill`, `update`, and `health_status*`), nudges the refresh worker.
3. The worker waits 200ms, drops duplicate nudges, and calls `RefreshOnce`.
4. Concurrent refreshes (the worker, the 30s collector, a handler that just mutated a container) share one in-flight `ContainerList`.
5. The UI already coalesces its own refetch. It does not open a daemon connection per tab.

`Ping` does not list containers. The engine container count is the size of the cache. Stats for the same container ID are fetched once and reused for one second.

## Frontend

In production the UI is the embedded `web/dist`. In development, Vite proxies `/api` (including WebSocket upgrades) to `127.0.0.1:8080`.

The UI gates on `GET /api/auth/me`, then `GET /api/auth/status`. Setup shows a token field. After login, one SSE connection drives refetches for containers, images, volumes, and networks, and feeds the Overview activity list (last 40 events, in memory). CPU and memory for all containers come from one `GET /api/stats` poller shared by every page, which also keeps a short in-memory history for the sparklines. The xterm terminal is a lazily loaded chunk.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `DOCKVISTA_ADDR` | `127.0.0.1:8080` | Listen address. The container image sets `:8080` |
| `DOCKVISTA_POLL_INTERVAL` | `30s` | Safety-net cache refresh |
| `DOCKVISTA_SHUTDOWN_TIMEOUT` | `10s` | Drain timeout |
| `DOCKVISTA_DATA_DIR` | `./data` | Credentials and session key. Gitignored |
| `DOCKVISTA_SETUP_TOKEN` | generated | Required by setup when no admin exists |
| `DOCKVISTA_COOKIE_SECURE` | `false` | Set `true` behind a TLS proxy |
| `DOCKVISTA_READ_ONLY` | `false` | Refuse every state-changing route and the terminal |

## Trust

Reaching an authenticated DockVista is equivalent to reaching the Docker socket. Bind mounts, environment variables, image pulls, and exec are not sandboxed. Mounting the socket read-only does not change that: `:ro` prevents replacing the socket file, not calling the API.

The setup token is what stops someone else on the network from creating the admin account first. Publishing port 8080 before that account exists exposes the setup form; the token is still required.

## Build

`make build` compiles the frontend into `internal/platform/embedweb/dist` and then the Go binary. `make dev` runs the API and Vite together. The container image is distroless, non-root, and expects the Docker socket plus a volume on `/data`.
