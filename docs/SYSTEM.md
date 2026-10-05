# DockVista system

DockVista is one process. It serves the web UI, talks to a Docker daemon, and keeps a single admin account. There is no database and no multi-user model.

## Runtime

`cmd/dockvista` loads configuration from the environment, opens a Docker client (`DOCKER_HOST` and the usual Docker variables), and wires the adapters:

1. A container service with an in-memory cache, plus a collector that refreshes that cache on `DOCKVISTA_POLL_INTERVAL` (default 30s).
2. An event bridge. One goroutine reads the daemon event stream and publishes each event to an in-process broker. Browser tabs subscribe over `GET /api/events` (SSE). The bridge does not list containers. A second goroutine coalesces "the list changed" signals and performs one `ContainerList` per burst.
3. Auth: credentials and the HMAC key live under `DOCKVISTA_DATA_DIR`.
4. Image, volume, and network services call the daemon directly. They are not cached.
5. HTTP on `DOCKVISTA_ADDR` (default `:8080`). The same server embeds `web/dist` for every non-API route.

Shutdown on SIGINT/SIGTERM drains in-flight requests for `DOCKVISTA_SHUTDOWN_TIMEOUT`.

## Layout

| Path | Role |
| --- | --- |
| `cmd/dockvista` | Process entry, dependency wiring, event bridge |
| `internal/core/domain` | Types only. No Docker SDK, no `net/http` |
| `internal/core/ports` | Interfaces the services depend on |
| `internal/core/service` | Use cases. Tested with fakes |
| `internal/adapters/docker` | The only package that imports the Docker SDK |
| `internal/adapters/store` | Mutex-guarded container cache. Lookup by full ID, name, or a unique prefix of at least 12 characters |
| `internal/adapters/authstore` | `credentials.json` and `session_secret` |
| `internal/adapters/broker` | Pub/sub. A slow subscriber drops events instead of blocking the rest |
| `internal/adapters/httpapi` | REST, SSE, WebSocket, validation, auth, rate limits |
| `internal/platform/embedweb` | Embeds the built frontend |
| `web` | React + TypeScript UI |

`internal/core` never imports an adapter. Handlers depend on small interfaces declared next to the handler, not on the concrete service type.

## HTTP

Public routes: `GET /healthz`, `GET /api/auth/status`, `POST /api/auth/setup`, `POST /api/auth/login`.

Everything else under `/api/` requires the session cookie. New routes added under that mux are authenticated by default.

| Area | Behavior |
| --- | --- |
| Containers | List comes from the cache. Start, stop, pause, unpause, restart, remove, create, inspect, and stats hit the daemon. Create tries to start; if start fails the response still returns the new ID (`started: false`) and the cache is refreshed |
| Logs | `GET /api/containers/{id}/logs` is SSE. TTY containers are read as a raw stream. Other containers are demultiplexed from Docker's framed stdout/stderr |
| Terminal | `GET /api/containers/{id}/exec` upgrades to a WebSocket and attaches `/bin/sh`. Same-origin is required |
| Images, volumes, networks | List, create, remove, prune. Image pull streams daemon progress as SSE |
| Events | `GET /api/events` is one broker subscription per tab |

Mutating routes are rate-limited (30 requests per minute per client address). Login and setup are limited to 5 per minute. Logs, events, and exec share a cap of 32 open streams.

Responses set `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, and `Referrer-Policy: no-referrer`.

## Auth

There is no default account in the binary.

On first launch, if `DOCKVISTA_SETUP_TOKEN` is unset, the process generates a token and logs it once at WARN. `POST /api/auth/setup` accepts that token a single time, with a username of 3–32 characters (`[A-Za-z0-9_.-]`) and a password of at least 8 characters. The password is stored bcrypt-hashed in `<DOCKVISTA_DATA_DIR>/credentials.json`. The directory is created mode `0700` and the file mode `0600` on Unix. Windows does not apply those modes.

Sessions are HMAC-SHA256 cookies named `dockvista_session` (`HttpOnly`, `SameSite=Strict`, 8 hours). The token includes a generation counter stored with the account. `POST /api/auth/logout` increments the counter, so every previously issued cookie stops verifying. Set `DOCKVISTA_COOKIE_SECURE=true` when a proxy terminates TLS in front of the process. The process itself does not serve TLS.

One account per instance. No roles.

## How a container change reaches the UI

1. The daemon emits an event.
2. The bridge publishes it to every SSE subscriber and, for list-changing actions (`create`, `start`, `stop`, `die`, `destroy`, `pause`, `unpause`, `restart`, `rename`, `kill`, `update`, and `health_status*`), nudges the refresh worker.
3. The worker waits 200ms, drops duplicate nudges, and calls `RefreshOnce`.
4. Concurrent refreshes (the worker, the 30s collector, a handler that just mutated a container) share one in-flight `ContainerList`.
5. The UI already coalesces its own refetch. It does not open a daemon connection per tab.

`Ping` does not list containers. The engine container count is the size of the cache. Stats for the same container ID are fetched once and reused for one second.

## Frontend

In production the UI is the embedded `web/dist`. In development, Vite proxies `/api` (including WebSocket upgrades) to `localhost:8080`.

The UI gates on `GET /api/auth/me`, then `GET /api/auth/status`. Setup shows a token field. After login, one SSE connection drives refetches for containers, images, volumes, and networks. Container CPU and memory are polled from `GET /api/containers/{id}/stats`.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `DOCKVISTA_ADDR` | `:8080` | Listen address |
| `DOCKVISTA_POLL_INTERVAL` | `30s` | Safety-net cache refresh |
| `DOCKVISTA_SHUTDOWN_TIMEOUT` | `10s` | Drain timeout |
| `DOCKVISTA_DATA_DIR` | `./data` | Credentials and session key. Gitignored |
| `DOCKVISTA_SETUP_TOKEN` | generated | Required by setup when no admin exists |
| `DOCKVISTA_COOKIE_SECURE` | `false` | Set `true` behind a TLS proxy |

## Trust

Reaching an authenticated DockVista is equivalent to reaching the Docker socket. Bind mounts, environment variables, image pulls, and exec are not sandboxed. Mounting the socket read-only does not change that: `:ro` prevents replacing the socket file, not calling the API.

The setup token is what stops someone else on the network from creating the admin account first. Publishing port 8080 before that account exists exposes the setup form; the token is still required.

## Build

`make build` compiles the frontend into `internal/platform/embedweb/dist` and then the Go binary. `make dev` runs the API and Vite together. The container image is distroless, non-root, and expects the Docker socket plus a volume on `/data`.
