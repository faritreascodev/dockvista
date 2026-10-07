# syntax=docker/dockerfile:1

# ---- frontend build -------------------------------------------------------
FROM node:22-alpine AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- backend build ----------------------------------------------------------
FROM golang:1.26-alpine AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-build /src/internal/platform/embedweb/dist ./internal/platform/embedweb/dist
RUN CGO_ENABLED=0 go build -o /out/dockvista ./cmd/dockvista
# distroless:nonroot has no shell to mkdir/chown at container runtime, so the
# credentials/session-secret data dir is prepared here and owned by uid:gid
# 65532 (nonroot's identity in the distroless image) ahead of time.
RUN mkdir -p /data && chown 65532:65532 /data

# ---- runtime ----------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go-build /out/dockvista /usr/local/bin/dockvista
# --chown is required: a plain COPY writes root-owned files, and the
# nonroot process could then not create the session secret.
COPY --from=go-build --chown=65532:65532 /data /data
ENV DOCKVISTA_DATA_DIR=/data
# The binary defaults to loopback, which is unreachable through a published
# port. Inside the container the network namespace is the boundary, so bind
# all interfaces here and restrict exposure with `-p 127.0.0.1:8080:8080`.
ENV DOCKVISTA_ADDR=:8080
EXPOSE 8080
# Distroless has no shell or curl; the binary probes itself. /readyz pings
# the Docker daemon; /healthz (process up) is a separate public route.
HEALTHCHECK --interval=15s --timeout=5s --start-period=15s --retries=3 \
  CMD ["/usr/local/bin/dockvista", "readyz"]
ENTRYPOINT ["/usr/local/bin/dockvista"]

# Run with the host's Docker socket mounted and a named volume for /data so
# the admin account survives a container recreate. The socket mount is how
# the process talks to the daemon; marking it :ro does not reduce what the
# Docker API can do. The image runs as a non-root user, which by default
# gets "permission denied" against a socket owned by the host's docker
# group — pass that group's GID explicitly instead of loosening the
# socket's permissions (GNU stat shown; on macOS hosts use `stat -f '%g'`,
# and Docker Desktop's socket is GID 0):
#   docker run -p 127.0.0.1:8080:8080 \
#     --group-add "$(stat -c '%g' /var/run/docker.sock)" \
#     -v /var/run/docker.sock:/var/run/docker.sock:ro \
#     -v dockvista-data:/data dockvista
