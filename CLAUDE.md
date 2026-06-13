# Probably — AI Agent Guide

## Environment setup (do this first)

All secrets live in 1Password ("Probably" vault). The `.env.1p` file in this repo contains `op://` references — never real values.

**To get a working `.env`:**

1. Check whether `OP_SERVICE_ACCOUNT_TOKEN` is already set in the environment.
   - If yes: `make env` will use it automatically.
   - If no: ask the user for the token, then run `make env`, or use the personal account (`op inject -i .env.1p -o .env --account my.1password.com`).

```sh
make env   # creates .env from .env.1p via 1Password
```

`make dev` will also run `make env` automatically if `.env` is missing.

## Running the app

```sh
make dev          # app on :8080 + worker on :8081 (hot reload via air)
make dev-app      # app only
make dev-worker   # worker only
make test         # unit tests
make test-integration  # requires Postgres running (make db-up)
```

Postgres is started by `make dev` via Docker when `docker compose` works. The app always connects to PostgreSQL using `DATABASE_URL` (e.g. in `.env`); there is no bundled database.

## Architecture

Go backend, HTMX frontend, PostgreSQL. No separate frontend build step for the main app.

```
cmd/server/          # HTTP server entrypoint
cmd/mcp-server/      # MCP server (ChatGPT/Claude integration)
cmd/ui-bundles/      # Vite-bundled widgets (MCP UI only)
internal/
  handlers/          # HTTP handlers (HTMX pages + JSON API)
  models/            # Domain types
  sync/              # Bank sync — Plaid, Teller, Akahu
  chat/              # AI chat
  llm/               # LLM executor + tool calls
  orchestrator/      # Multi-model orchestrator
  mcp/               # MCP server implementation
  insights/          # Financial insights engine
  pulse/             # Spending pace calculator
  recurring/         # Recurring transaction detector
  enrichment/        # Transaction enrichment (logos, currency)
  processing/        # Background processing worker
  categorize/        # LLM-based categorisation
  auth/              # WebAuthn / passkey auth
  billing/           # Stripe billing
  db/                # DB layer + migrations
  views/             # HTMX templates (Go html/template)
static/              # CSS, icons, MCP UI bundles
```

## Key conventions

- Structured logging via `slog` throughout — no `fmt.Println` in production paths.
- Handlers receive dependencies via a `*Handlers` struct, not global state.
- Database migrations live in `internal/db/migrations/` and run automatically on startup.
- Tests that need Postgres use `TEST_DATABASE_URL`; unit tests mock at the interface boundary.
- CSS is built with Tailwind — edit `static/input.css`, run `make css`.
