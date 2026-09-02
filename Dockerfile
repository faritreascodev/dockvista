# syntax=docker/dockerfile:1

# ---- frontend build -------------------------------------------------------
FROM node:22-alpine AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- backend build ----------------------------------------------------------
FROM golang:1.25-alpine AS go-build
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
COPY --from=go-build /data /data
ENV DOCKVISTA_DATA_DIR=/data
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/dockvista"]

# Run with the host's Docker socket mounted and a named volume for /data so
# the admin account survives a container recreate. The image runs as a
# non-root user, which by default gets "permission denied" against a socket
# owned by the host's docker group — pass that group's GID explicitly
# instead of loosening the socket's permissions:
#   docker run -p 8080:8080 \
#     --group-add "$(stat -c '%g' /var/run/docker.sock)" \
#     -v /var/run/docker.sock:/var/run/docker.sock:ro \
#     -v dockvista-data:/data dockvista
