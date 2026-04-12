# waxon — slide deck toolkit

default:
    @just --list

# Run from source
run *args:
    go run ./cmd/waxon {{args}}

# Build the binary
build:
    go build -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o waxon ./cmd/waxon

# Run tests
test:
    go test ./...

# Run linter
lint:
    golangci-lint run ./...

# Format code
format:
    gofmt -w cmd/ internal/

# Quality checks
check: format lint test

# Start dev server with live reload (via overmind + air)
dev:
    overmind start -f Procfile.dev

# Run tests with coverage
cover:
    go test ./... -cover

# Run tests with detailed coverage report
cover-report:
    go test ./... -coverprofile=coverage.out
    go tool cover -func=coverage.out

# Set up git hooks
setup:
    git config core.hooksPath .githooks

# Clean build artifacts
clean:
    rm -f waxon
    rm -rf dist/goreleaser/
    rm -rf tmp/
    rm -f coverage.out
