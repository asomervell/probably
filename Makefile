# Probably - Makefile
# Build targets for both web server and desktop app

.PHONY: all dev build clean server desktop desktop-dev desktop-build deps db-up env help _dev-inner build-app build-worker build-dev

# Resolve the `air` binary even when GOPATH/bin isn't on PATH.
AIR := $(shell which air 2>/dev/null || echo $(shell go env GOPATH)/bin/air)

# Default target
all: help

# ============================================================================
# Secrets (1Password)
# ============================================================================

env:
	@echo "🔐 Injecting secrets from 1Password into .env..."
	@if [ ! -f .env.1p ]; then echo "❌ .env.1p not found" && exit 1; fi
	@if [ -n "$$OP_SERVICE_ACCOUNT_TOKEN" ]; then \
		op inject -i .env.1p -o .env; \
	else \
		op inject -i .env.1p -o .env --account my.1password.com; \
	fi
	@echo "✅ .env created from .env.1p"

# ============================================================================
# Dependencies
# ============================================================================

deps:
	@echo "📦 Installing Go dependencies..."
	go mod download
	go mod tidy

deps-desktop: deps
	@echo "📦 Checking Wails CLI..."
	@which wails > /dev/null || (echo "❌ Wails CLI not found. Install with: go install github.com/wailsapp/wails/v2/cmd/wails@latest" && exit 1)

# ============================================================================
# Database
# ============================================================================

db-up:
	@echo "🐘 Ensuring PostgreSQL is running..."
	@docker compose up db -d 2>/dev/null && echo "✅ Docker PostgreSQL is ready" || \
		(echo "❌ Could not start Docker PostgreSQL. Install Docker or start PostgreSQL and set DATABASE_URL in .env" && exit 1)

# Run goose migrations (uses DATABASE_URL from .env)
db-migrate:
	@echo "📂 Running database migrations..."
	go run ./cmd/util migrate

# ============================================================================
# Web Server (existing workflow)
# ============================================================================

# Dev binaries that `air` watches. We compile these; air does NOT build — it
# only watches the resulting binary and restarts the process when it changes.
# Rebuild after a change (e.g. `make build-app`) and the running server bounces.
build-app:
	@echo "🔨 Building app binary (./bin/probably-app)..."
	@mkdir -p bin
	@[ -f ./bin/tailwindcss ] && ./bin/tailwindcss -i ./static/input.css -o ./static/output.css || true
	go build -o ./bin/probably-app ./cmd/server

build-worker:
	@echo "🔨 Building worker binary (./bin/probably-worker)..."
	@mkdir -p bin
	go build -o ./bin/probably-worker ./cmd/server

# Rebuild both dev binaries; any running `air` processes restart automatically.
build-dev: build-app build-worker

dev:
	@if which humanlog > /dev/null 2>&1; then \
		$(MAKE) _dev-inner 2>&1 | humanlog; \
	else \
		echo "💡 Tip: Install humanlog for pretty JSON logs: brew install humanlog"; \
		$(MAKE) _dev-inner; \
	fi

_dev-inner:
	@[ -f .env ] || $(MAKE) env
	@echo "🛑 Stopping any existing services..."
	@OLD_PIDS=$$(lsof -ti:8080 2>/dev/null | tr '\n' ' '); \
	if [ -n "$$OLD_PIDS" ]; then \
		echo "   Killing processes on port 8080: $$OLD_PIDS"; \
		echo $$OLD_PIDS | xargs kill -9 2>/dev/null || true; \
	fi; \
	OLD_PIDS=$$(lsof -ti:8081 2>/dev/null | tr '\n' ' '); \
	if [ -n "$$OLD_PIDS" ]; then \
		echo "   Killing processes on port 8081: $$OLD_PIDS"; \
		echo $$OLD_PIDS | xargs kill -9 2>/dev/null || true; \
	fi; \
	pkill -9 -f "^air " 2>/dev/null || true; \
	pkill -9 -f "probably-(server|app|worker)" 2>/dev/null || true; \
	pkill -9 -f "go run.*cmd/server" 2>/dev/null || true; \
	pkill -9 -f "ngrok" 2>/dev/null || true; \
	sleep 2; \
	echo "✅ Existing services stopped"
	@echo "💡 Using DATABASE_URL from .env file"
	@echo "🐘 Starting PostgreSQL in background..."
	@docker compose up db -d 2>/dev/null || echo "⚠️  Could not start Docker PostgreSQL (may already be running or permission issue)"
	@echo "🚀 Starting application server (port 8080) - starting immediately..."
	@trap "pkill -9 -f 'air.*-c.*\.air\.' 2>/dev/null || true; pkill -9 -f 'ngrok' 2>/dev/null || true; pkill -9 -f 'stripe listen' 2>/dev/null || true" EXIT INT TERM; \
	$(MAKE) build-app; \
	RUNTIME_MODE=app $(AIR) -c .air.app.toml & \
	APP_PID=$$!; \
	echo "✅ App server starting (PID: $$APP_PID)"; \
	if which ngrok > /dev/null 2>&1; then \
		echo "🌐 Starting ngrok tunnel in background..."; \
		NGROK_DOMAIN=""; \
		if [ -f .env ]; then \
			NGROK_DOMAIN=$$(grep -E '^[[:space:]]*NGROK_DOMAIN=' .env | tail -1 | cut -d= -f2- | tr -d '\r' | sed 's/^[[:space:]]*//;s/[[:space:]]*$$//'); \
		fi; \
		if [ -n "$$NGROK_DOMAIN" ]; then \
			echo "   Using reserved host: $$NGROK_DOMAIN"; \
			ngrok http 8080 --domain="$$NGROK_DOMAIN" --log=stdout > /tmp/ngrok.log 2>&1 & \
		else \
			ngrok http 8080 --log=stdout > /tmp/ngrok.log 2>&1 & \
		fi; \
		sleep 2; \
		echo "   ngrok running"; \
		echo "   Check /tmp/ngrok.log for the public URL"; \
		echo "   Or visit http://localhost:4040 for ngrok web interface"; \
	else \
		echo "⚠️  ngrok not found. Install with: brew install ngrok/ngrok/ngrok"; \
		echo "   Public tunnel will not be available. Continuing anyway..."; \
	fi; \
	if which stripe > /dev/null 2>&1; then \
		echo "🔔 Starting Stripe webhook forwarding in background..."; \
		stripe listen --forward-to localhost:8080/api/stripe/webhook & \
		STRIPE_PID=$$!; \
		echo "   Stripe webhook forwarding started (PID: $$STRIPE_PID)"; \
	else \
		echo "⚠️  Stripe CLI not found. Install with: brew install stripe/stripe-cli/stripe"; \
		echo "   Webhooks will not be forwarded. Continuing anyway..."; \
	fi; \
	echo "⏳ Waiting for app to be ready before starting worker..."; \
	MAX_WAIT=30; \
	WAIT_COUNT=0; \
	while [ $$WAIT_COUNT -lt $$MAX_WAIT ]; do \
		if curl -sf http://localhost:8080 > /dev/null 2>&1; then \
			echo "✅ App is ready!"; \
			break; \
		fi; \
		WAIT_COUNT=$$((WAIT_COUNT + 1)); \
		if [ $$WAIT_COUNT -eq $$MAX_WAIT ]; then \
			echo "❌ App failed to start within $$MAX_WAIT seconds"; \
			echo "   Check logs above for errors"; \
			exit 1; \
		fi; \
		sleep 1; \
		echo "   Waiting for app... ($$WAIT_COUNT/$$MAX_WAIT)"; \
	done; \
	echo "🔄 Starting worker (port 8081) for background processing..."; \
	$(MAKE) build-worker; \
	RUNTIME_MODE=worker $(AIR) -c .air.worker.toml & \
	WORKER_PID=$$!; \
	echo "✅ Worker starting (PID: $$WORKER_PID)"; \
	echo ""; \
	echo "🎉 All services started!"; \
	echo "   App: http://localhost:8080 (PID: $$APP_PID)"; \
	echo "   Worker: port 8081 (PID: $$WORKER_PID)"; \
	wait $$APP_PID $$WORKER_PID; \
	pkill -9 -f 'ngrok' 2>/dev/null || true; \
	pkill -9 -f 'stripe listen' 2>/dev/null || true

dev-app: build-app
	@echo "🛑 Stopping any existing services on port 8080..."
	@lsof -ti:8080 | xargs kill -9 2>/dev/null || true
	@echo "🚀 Starting application server only (air watches the binary)..."
	@RUNTIME_MODE=app $(AIR) -c .air.app.toml

dev-worker: build-worker
	@echo "🛑 Stopping any existing services on port 8081..."
	@lsof -ti:8081 | xargs kill -9 2>/dev/null || true
	@echo "🔄 Starting worker only (air watches the binary)..."
	@RUNTIME_MODE=worker $(AIR) -c .air.worker.toml

dev-no-reload:
	@echo "🚀 Starting web server in development mode..."
	@echo "💡 Set DATABASE_URL in .env (e.g. after make db-up) before starting the server"
	go run ./cmd/server

dev-mcp-standalone:
	@echo "🚀 Starting MCP server standalone (port 8081)..."
	@echo "💡 Tip: MCP server is also integrated into main server at /mcp"
	@echo "💡 Use this for testing MCP server independently"
	go run ./cmd/mcp-server

server: build-server
	@echo "🚀 Running web server..."
	./bin/probably-server

build-server:
	@echo "🔨 Building web server..."
	@mkdir -p bin
	go build -o bin/probably-server ./cmd/server

build-mcp-server:
	@echo "🔨 Building MCP server..."
	@mkdir -p bin
	go build -o bin/probably-mcp-server ./cmd/mcp-server

build-util:
	@echo "🔨 Building utility CLI..."
	@mkdir -p bin
	go build -o bin/util ./cmd/util

# ============================================================================
# Desktop App (Wails)
# ============================================================================

desktop-dev: deps-desktop
	@echo "🖥️  Starting desktop app in development mode..."
	wails dev

desktop-build: deps-desktop
	@echo "🔨 Building desktop app..."
	wails build

desktop-build-prod: deps-desktop
	@echo "🔨 Building production desktop app..."
	wails build -production -platform darwin/universal

# Platform-specific builds
desktop-build-mac: deps-desktop
	@echo "🍎 Building for macOS..."
	wails build -platform darwin/universal

desktop-build-mac-arm: deps-desktop
	@echo "🍎 Building for macOS (Apple Silicon)..."
	wails build -platform darwin/arm64

desktop-build-mac-intel: deps-desktop
	@echo "🍎 Building for macOS (Intel)..."
	wails build -platform darwin/amd64

desktop-build-windows: deps-desktop
	@echo "🪟 Building for Windows..."
	wails build -platform windows/amd64

desktop-build-linux: deps-desktop
	@echo "🐧 Building for Linux..."
	wails build -platform linux/amd64

# ============================================================================
# CSS (Tailwind)
# ============================================================================

css:
	@echo "🎨 Building CSS..."
	@mkdir -p bin
	./bin/tailwindcss -i ./static/input.css -o ./static/output.css --minify

css-watch:
	@echo "👀 Watching CSS for changes..."
	@mkdir -p bin
	./bin/tailwindcss -i ./static/input.css -o ./static/output.css --watch

css-install:
	@echo "📦 Installing Tailwind CSS binary..."
	@mkdir -p bin
	@if [ "$$(uname -m)" = "arm64" ]; then \
		curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-arm64; \
		chmod +x tailwindcss-macos-arm64; \
		mv tailwindcss-macos-arm64 bin/tailwindcss; \
	else \
		curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-x64; \
		chmod +x tailwindcss-macos-x64; \
		mv tailwindcss-macos-x64 bin/tailwindcss; \
	fi
	@echo "✅ Tailwind CSS installed to bin/"

# ============================================================================
# Utilities
# ============================================================================

clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -rf build/bin/
	rm -rf dist/

# Generate Wails bindings
generate:
	@echo "⚙️  Generating Wails bindings..."
	wails generate module

# Run tests
test:
	@echo "🧪 Running unit tests..."
	go test -v ./...

# Run integration tests (requires database)
test-integration:
	@echo "🧪 Running integration tests..."
	@echo "💡 Requires PostgreSQL running (make db-up)"
	TEST_DATABASE_URL=postgres://finance:secret@localhost:5432/finance?sslmode=disable go test -v ./...

# Run linter
lint:
	@echo "🔍 Running linter..."
	golangci-lint run

# ============================================================================
# Help
# ============================================================================

help:
	@echo ""
	@echo "Probably - Build Commands"
	@echo "========================="
	@echo ""
	@echo "Setup:"
	@echo "  make env              - Create .env by injecting secrets from 1Password"
	@echo ""
	@echo "Web Server:"
	@echo "  make dev              - Run app (8080) + worker (8081) in dev mode"
	@echo "  make dev-app          - Run app server only (with Tailwind, port 8080)"
	@echo "  make dev-worker       - Run worker only (no Tailwind, port 8081)"
	@echo "  make dev-mcp-standalone - Run MCP server standalone (port 8081, for testing)"
	@echo "  make build-server    - Build web server binary"
	@echo "  make build-mcp-server - Build MCP server binary"
	@echo "  make server          - Build and run web server"
	@echo "  make build-util      - Build utility CLI binary"
	@echo ""
	@echo "Desktop App:"
	@echo "  make desktop-dev     - Run desktop app in dev mode (hot reload)"
	@echo "  make desktop-build   - Build desktop app for current platform"
	@echo "  make desktop-build-prod - Build production desktop app"
	@echo ""
	@echo "Platform-Specific Desktop Builds:"
	@echo "  make desktop-build-mac       - Build for macOS (Universal)"
	@echo "  make desktop-build-mac-arm   - Build for macOS (Apple Silicon)"
	@echo "  make desktop-build-mac-intel - Build for macOS (Intel)"
	@echo "  make desktop-build-windows   - Build for Windows"
	@echo "  make desktop-build-linux     - Build for Linux"
	@echo ""
	@echo "CSS:"
	@echo "  make css             - Build CSS with Tailwind"
	@echo "  make css-watch       - Watch and rebuild CSS on changes"
	@echo "  make css-install     - Download Tailwind binary"
	@echo ""
	@echo "Database:"
	@echo "  make db-up           - Start PostgreSQL container"
	@echo "  make db-migrate      - Run goose migrations (DATABASE_URL in .env)"
	@echo "  (go run ./cmd/util plaid-sync-accounts)  Re-sync Plaid after migrations"
	@echo ""
	@echo "Utilities:"
	@echo "  make deps            - Install Go dependencies"
	@echo "  make clean           - Remove build artifacts"
	@echo "  make test            - Run tests"
	@echo "  make lint            - Run linter"
	@echo ""

