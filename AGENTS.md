# AGENTS.md

> Instructions for AI agents (Claude Code, Cursor, Copilot, etc.) working in this repo.

## Cloud Environment Quick Start

```bash
bash scripts/cloud-setup.sh
```

This single command:
1. Creates `.env` from `.env.1p` via 1Password if missing
2. Builds the server binary
3. Starts ngrok for public webhook URLs
4. Runs all database migrations
5. Seeds the dev user (idempotent)
6. Starts the application server on port 8080

## Secrets & Environment

All secrets live in 1Password ("Probably" vault). The `.env.1p` file (committed) contains `op://` references — never real values.

**To create `.env`:**

1. Check if `OP_SERVICE_ACCOUNT_TOKEN` is set. If not, ask the user for it.
2. Run:

```bash
make env   # injects secrets from 1Password into .env
```

`make dev` runs `make env` automatically if `.env` is missing.
When `OP_SERVICE_ACCOUNT_TOKEN` is set it is used automatically; otherwise the personal account at `my.1password.com` is used.

## Dev User Credentials

| Field    | Value               |
|----------|---------------------|
| Email    | `dev@probably.test` |
| Password | `devsecret1`        |
| URL      | http://localhost:8080 (or ngrok URL below) |

Seed manually at any time:
```bash
go run ./cmd/util seed
```

## ngrok — Public Webhook URLs

Get the current public URL:
```bash
bash scripts/ngrok-url.sh
# → https://abc123.ngrok-free.app
```

For a stable domain, set `NGROK_DOMAIN` and `NGROK_AUTHTOKEN` in `.env`:
```
NGROK_AUTHTOKEN=your-token-here
NGROK_DOMAIN=your-name.ngrok-free.dev
```

`cloud-setup.sh` auto-writes these webhook URLs to `.env` when ngrok is running:

| Provider | Endpoint                        |
|----------|---------------------------------|
| Plaid    | `$NGROK_URL/api/plaid/webhook`  |
| Teller   | `$NGROK_URL/api/teller/webhook` |
| Akahu    | `$NGROK_URL/api/akahu/webhook`  |
| Stripe   | `$NGROK_URL/api/stripe/webhook` |

ngrok dashboard: http://localhost:4040
ngrok logs: `/tmp/ngrok.log`

## Key Commands

| Command                          | Description                              |
|----------------------------------|------------------------------------------|
| `bash scripts/cloud-setup.sh`   | Full cloud environment setup             |
| `bash scripts/ngrok-url.sh`     | Print current public ngrok URL           |
| `make dev`                       | App + worker with hot reload (local dev) |
| `make dev-app`                   | App server only with hot reload          |
| `go run ./cmd/util migrate`      | Run database migrations                  |
| `go run ./cmd/util seed`         | Seed dev user (idempotent)               |
| `go run ./cmd/util migrate-status` | Show migration status                  |
| `go run ./cmd/util help`         | List all util commands                   |

## App Architecture

| Layer     | Entry point                    | Port |
|-----------|--------------------------------|------|
| App server | `cmd/server/main.go`          | 8080 |
| Worker     | same binary, `RUNTIME_MODE=worker` | 8081 |
| MCP server | `cmd/mcp-server/main.go`      | 8081 |
| Util CLI   | `cmd/util/main.go`            | —    |

Logs:
- Server: `/tmp/probably-server.log`
- Worker: `/tmp/probably-worker.log`

## Database

- Migrations: `internal/db/migrations/` (numbered `NNN_description.sql`)
- Driver: goose + PostgreSQL
- Cloud / CI: Docker postgres (`docker compose up db -d`), `DATABASE_URL=postgres://finance:secret@localhost:5432/finance?sslmode=disable`
- `cloud-setup.sh` starts the container and writes `DATABASE_URL` to `.env` automatically
- Migrations run via `go run ./cmd/util migrate` (or automatically on server start when `DATABASE_URL` is set)

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):
```
feat: add logo enrichment for transactions
fix: resolve transfer matching edge case
feat!: change account model structure   ← breaking change
```

Types that trigger a release: `feat`, `fix`, `feat!`, `fix!`.

## Git

Standard `git` operations — no special signing or SSH agent required in cloud environments.
Push to the current feature branch and open a PR against `main`.
