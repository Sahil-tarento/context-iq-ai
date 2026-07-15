.PHONY: build build-mcp install clean test

## ─── Build ───────────────────────────────────────────────────────────────────

build: build-daemon build-cli build-mcp

build-daemon:
	go build -o contextiq ./cmd/contextiq/

build-cli:
	go build -o contextiq-cli ./cmd/cli/

build-mcp:
	go build -o contextiq-mcp ./cmd/contextiq-mcp/

## ─── Install (adds all binaries to /usr/local/bin) ──────────────────────────

install: build
	bash scripts/install.sh

## ─── Go install (adds to GOPATH/bin — works without sudo) ───────────────────

go-install:
	go install ./cmd/contextiq/
	go install ./cmd/cli/
	go install ./cmd/contextiq-mcp/

## ─── Run ─────────────────────────────────────────────────────────────────────

run:
	./contextiq --port 9009

run-docker:
	docker compose up -d

## ─── Test ────────────────────────────────────────────────────────────────────

test:
	go test ./...

## ─── Clean ───────────────────────────────────────────────────────────────────

clean:
	rm -f contextiq contextiq-cli contextiq-mcp
