# waxon — slide deck toolkit

VERSION    := `git describe --tags --always --dirty 2>/dev/null || echo dev`
GIT_COMMIT := `git rev-parse --short HEAD 2>/dev/null || echo unknown`
DIRTY      := `test -z "$(git status --porcelain 2>/dev/null)" && echo false || echo true`
LDFLAGS    := "-s -w" + \
              " -X github.com/mschulkind-oss/waxon/internal/version.Version=" + VERSION + \
              " -X github.com/mschulkind-oss/waxon/internal/version.GitCommit=" + GIT_COMMIT + \
              " -X github.com/mschulkind-oss/waxon/internal/version.Dirty=" + DIRTY

default:
    @just --list

# Install everything needed to build and run waxon (idempotent)
setup:
    #!/usr/bin/env bash
    set -euo pipefail
    if command -v mise >/dev/null 2>&1; then
        echo "→ Installing toolchain via mise..."
        mise install
    else
        echo "→ mise not found; assuming go is on PATH"
        command -v go >/dev/null || { echo "go not installed; install Go 1.26+ first"; exit 1; }
    fi
    echo "→ Downloading Go modules..."
    go mod download
    git config core.hooksPath .githooks
    echo "✓ setup complete"

# Run from source
run *args:
    go run ./cmd/waxon {{args}}

# Build the binary into ./dist
build:
    @mkdir -p dist
    go build -ldflags '{{LDFLAGS}}' -o dist/waxon ./cmd/waxon

# Install the built binary into ~/.local/bin
install: build
    @mkdir -p ~/.local/bin
    rm -f ~/.local/bin/waxon
    cp dist/waxon ~/.local/bin/waxon
    @echo "✓ installed: ~/.local/bin/waxon"

# Build, install, and (if present) restart the user service
deploy: install
    #!/usr/bin/env bash
    set -euo pipefail
    case "$(uname -s)" in
        Linux)
            if systemctl --user list-unit-files waxon.service &>/dev/null && \
               systemctl --user list-unit-files waxon.service | grep -q waxon.service; then
                systemctl --user restart waxon && echo "✓ waxon service restarted"
            fi
            ;;
        Darwin)
            plist="$HOME/Library/LaunchAgents/dev.waxon.waxon.plist"
            if [ -f "$plist" ]; then
                launchctl unload "$plist" 2>/dev/null || true
                launchctl load "$plist" && echo "✓ waxon service restarted"
            fi
            ;;
    esac
    echo "✓ waxon deployed. Verify: waxon --version"

# Run tests
test *ARGS:
    go test ./... {{ARGS}}

# Run tests with coverage
cover:
    go test ./... -cover

# Run tests with detailed coverage report
cover-report:
    go test ./... -coverprofile=coverage.out
    go tool cover -func=coverage.out

# Run linter
lint:
    golangci-lint run ./...

# Format code
format:
    gofmt -w cmd/ internal/

# Quality checks (run before committing)
check: format lint test

# Start dev server with live reload
dev:
    overmind start -f Procfile.dev

# Restart the user service (systemd on Linux, launchd on macOS)
restart-service:
    waxon service restart

# Show user service status
status-service:
    waxon service status

# Tail user service logs
logs:
    waxon service logs

# Clean build artifacts (preserves package source dirs in dist/)
clean:
    rm -f dist/waxon
    rm -rf dist/goreleaser/
    rm -rf tmp/
    rm -f coverage.out
