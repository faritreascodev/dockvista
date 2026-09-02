.PHONY: build dev frontend-build backend-build test lint tidy docker-build clean

BINARY := dockvista
DIST_DIR := internal/platform/embedweb/dist
# Explicit package set (not ./...): web/node_modules can contain stray
# vendored .go files from npm packages, and there's no clean way to make
# `./...` skip a non-module subdirectory.
GO_PACKAGES := ./cmd/... ./internal/... ./pkg/...

## build: build the frontend, embed it, and produce the single dockvista binary
build: frontend-build backend-build

## dev: run the Go API and the Vite dev server together (frontend proxies /api to :8080)
dev:
	@echo "Starting API on :8080 and Vite dev server..."
	@( trap 'kill 0' EXIT; \
		go run ./cmd/dockvista & \
		npm --prefix web run dev & \
		wait )

## frontend-build: build the React/TS frontend into $(DIST_DIR)
frontend-build:
	npm --prefix web ci
	npm --prefix web run build

## backend-build: compile the Go binary (requires $(DIST_DIR) to be populated)
backend-build:
	go build -o bin/$(BINARY) ./cmd/dockvista

## test: run the Go test suite with race detection and coverage
test:
	go test $(GO_PACKAGES) -race -cover

## lint: run golangci-lint (Go) and eslint (frontend)
lint:
	golangci-lint run $(GO_PACKAGES)
	npm --prefix web run lint

## tidy: sync Go and npm dependency manifests
tidy:
	go mod tidy
	npm --prefix web install

## docker-build: build the DockVista container image
docker-build:
	docker build -t dockvista:local .

## clean: remove build artifacts
clean:
	rm -rf bin web/node_modules
