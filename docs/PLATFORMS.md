# Platform support

DockVista is one Go binary plus an embedded web UI, and it talks to a Docker
daemon. This page records what has been run, how to run it on each platform,
and what has not been verified.

## Verified

| Platform | Setup | What was run |
| --- | --- | --- |
| Windows 11 (native binary) | Docker Desktop 29.x, Go 1.27 | Binary built with the embedded UI, run against Docker Desktop over its named pipe. Setup, login, `/api/engine`, `/api/containers`. |
| Windows 11 + Docker Desktop (container) | `docker compose up -d --build` | Setup, login, daemon calls, container create, the 201/502 create contract, admin persistence across `down`/`up`. |
| Linux (Ubuntu 26.04 on WSL2, Docker Engine 29.1) | Non-root user in the `docker` group, `DOCKER_GID` set | Same flow as above, plus logout revoking the session. |
| Linux, `go test -race` | Go 1.26 | `go vet` and the full test suite with the race detector, the same as CI. |
| Cross-compilation | Go 1.27 | `linux`, `darwin`, and `windows` for `amd64` and `arm64`, with `CGO_ENABLED=0`. |

Not verified: running on macOS, running the Linux `arm64` or Windows `arm64`
binaries, and running the container image on `arm64`. Those builds compile,
but nothing was started.

## Docker

The Docker image and `docker-compose.yml` are the portable route. They need
only Docker; no Go or Node toolchain is required on the host.

```bash
docker compose up -d --build
```

The container runs as a non-root user, so it needs the group of the Docker
socket:

- **Docker Desktop (Windows, macOS):** the socket is owned by `root` (GID 0),
  which is the default in `docker-compose.yml`. Nothing to set.
- **Linux:** set `DOCKER_GID` to the host's `docker` group. Without it the app
  starts, but every daemon call fails with `permission denied` on the socket.

```bash
export DOCKER_GID="$(getent group docker | cut -d: -f3)"
docker compose up -d --build
```

## Native builds

Build the frontend first. The Go binary embeds `web/dist`, so a binary built
before the frontend shows only a placeholder page.

On Linux and macOS:

```bash
make build           # frontend, embed, then ./bin/dockvista
./bin/dockvista
```

On Windows PowerShell, without `make`:

```powershell
npm --prefix web ci
npm --prefix web run build
go build -o bin\dockvista.exe .\cmd\dockvista
.\bin\dockvista.exe
```

The `make dev` target runs under a POSIX shell (`trap`, `kill 0`). On Windows,
use WSL or the commands above, or use Docker Compose.

The native binary reads `DOCKER_HOST`. If it is unset, the Docker client uses
the platform default: the named pipe on Windows, the Unix socket elsewhere.

## Known limits

- The Compose page groups containers by project label. It does not run
  `docker compose up` or `down`; see the README roadmap.
- `DOCKVISTA_DATA_DIR` holds the admin credentials and session key. On Windows
  the POSIX file modes (`0700`, `0600`) are not applied, so protect that
  directory with NTFS permissions.
