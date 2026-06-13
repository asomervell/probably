#!/bin/bash
# Cloud development environment setup for Probably.
# Idempotent: safe to run multiple times.
# Uses system PostgreSQL (preferred) or Docker postgres as fallback.
# Starts ngrok (if available/configured), runs migrations, seeds dev user, starts server.

set -e

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_DIR"

PORT="${PORT:-8080}"
DEV_EMAIL="dev@probably.test"
DEV_PASSWORD="devsecret1"

# ---- colours ----
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'
info()    { echo -e "${BLUE}==>${NC} ${BOLD}$1${NC}"; }
success() { echo -e "${GREEN}==>${NC} ${BOLD}$1${NC}"; }
warn()    { echo -e "${YELLOW}==>${NC} ${BOLD}$1${NC}"; }

# ---- .env ----
if [ ! -f "$REPO_DIR/.env" ]; then
    if ! which op > /dev/null 2>&1; then
        warn "op CLI not found — install 1Password CLI to inject secrets"
        exit 1
    fi
    if [ -z "${OP_SERVICE_ACCOUNT_TOKEN:-}" ]; then
        warn "OP_SERVICE_ACCOUNT_TOKEN not set — set it in the Claude Code environment variables"
        exit 1
    fi
    info "Creating .env from 1Password..."
    op inject -i "$REPO_DIR/.env.1p" -o "$REPO_DIR/.env"
    success ".env created from 1Password"
fi

# Disable Stripe billing so the app starts without Stripe keys
if grep -q "^BILLING_ENABLED=true" "$REPO_DIR/.env"; then
    sed -i 's/^BILLING_ENABLED=true/BILLING_ENABLED=false/' "$REPO_DIR/.env"
fi

# Source .env so NGROK_DOMAIN / NGROK_AUTHTOKEN etc. are available
set -a
# shellcheck disable=SC1091
source "$REPO_DIR/.env" 2>/dev/null || true
set +a

# ---- postgres setup ----
# Prefer system PostgreSQL (no Docker needed), fall back to Docker.
DB_URL=""

start_system_postgres() {
    # Check if pg_ctlcluster is available (Debian/Ubuntu system postgres)
    if ! which pg_ctlcluster > /dev/null 2>&1; then
        return 1
    fi

    # Find the installed postgres version
    PG_VER=$(pg_lsclusters --no-header 2>/dev/null | awk '{print $1}' | head -1)
    PG_CLUSTER=$(pg_lsclusters --no-header 2>/dev/null | awk '{print $2}' | head -1)
    if [ -z "$PG_VER" ]; then
        return 1
    fi

    info "Using system PostgreSQL $PG_VER ($PG_CLUSTER)..."
    STATUS=$(pg_lsclusters --no-header 2>/dev/null | awk '{print $4}' | head -1)
    if [ "$STATUS" != "online" ]; then
        pg_ctlcluster "$PG_VER" "$PG_CLUSTER" start > /dev/null 2>&1 || return 1
        sleep 2
    fi

    # Create user+db if they don't exist (run as postgres OS user)
    su -c "psql -c \"CREATE USER finance WITH PASSWORD 'secret';\" 2>/dev/null; \
           psql -c \"CREATE DATABASE finance OWNER finance;\" 2>/dev/null; \
           psql -c \"GRANT ALL PRIVILEGES ON DATABASE finance TO finance;\" 2>/dev/null; \
           true" postgres 2>/dev/null || true

    DB_URL="postgres://finance:secret@localhost:5432/finance?sslmode=disable"
    return 0
}

start_docker_postgres() {
    if ! docker info > /dev/null 2>&1; then
        return 1
    fi

    CONTAINER_NAME="probably-db-dev"
    info "Starting PostgreSQL container ($CONTAINER_NAME)..."

    if docker container inspect "$CONTAINER_NAME" > /dev/null 2>&1; then
        if [ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null)" != "true" ]; then
            docker start "$CONTAINER_NAME" > /dev/null
        fi
    else
        docker run -d \
            --name "$CONTAINER_NAME" \
            -e POSTGRES_DB=finance \
            -e POSTGRES_USER=finance \
            -e POSTGRES_PASSWORD=secret \
            -p 5432:5432 \
            postgres:17-alpine > /dev/null
        success "Container started"
    fi

    # Wait for postgres to be ready
    MAX_PG_WAIT=60; PG_WAITED=0
    until docker exec "$CONTAINER_NAME" pg_isready -U finance -d finance > /dev/null 2>&1; do
        PG_WAITED=$((PG_WAITED + 1))
        [ "$PG_WAITED" -ge "$MAX_PG_WAIT" ] && warn "PostgreSQL did not become ready after ${MAX_PG_WAIT}s" && exit 1
        sleep 1
    done

    DB_URL="postgres://finance:secret@localhost:5432/finance?sslmode=disable"
    return 0
}

if start_system_postgres; then
    success "System PostgreSQL ready"
elif start_docker_postgres; then
    success "Docker PostgreSQL ready"
else
    warn "Neither system PostgreSQL nor Docker is available. Install one of them, or set DATABASE_URL in .env to a reachable PostgreSQL server."
    exit 1
fi

# Set DATABASE_URL in .env if we found a database
if [ -n "$DB_URL" ]; then
    if grep -q "^DATABASE_URL=" "$REPO_DIR/.env"; then
        sed -i "s|^DATABASE_URL=.*|DATABASE_URL=$DB_URL|" "$REPO_DIR/.env"
    else
        echo "DATABASE_URL=$DB_URL" >> "$REPO_DIR/.env"
    fi
    export DATABASE_URL="$DB_URL"
fi

# ---- ngrok ----
start_ngrok() {
    pkill -f "ngrok http" 2>/dev/null || true
    sleep 1

    NGROK_BIN=""
    if which ngrok > /dev/null 2>&1; then
        NGROK_BIN="ngrok"
    elif [ -x "/tmp/ngrok" ]; then
        NGROK_BIN="/tmp/ngrok"
    else
        warn "ngrok not found — webhook URLs will not be available"
        warn "To install: see https://ngrok.com/download"
        return 1
    fi

    if [ -n "${NGROK_AUTHTOKEN:-}" ]; then
        $NGROK_BIN config add-authtoken "$NGROK_AUTHTOKEN" > /dev/null 2>&1 || true
    fi

    local ngrok_args="http $PORT --log=stdout"
    if [ -n "${NGROK_DOMAIN:-}" ]; then
        ngrok_args="http $PORT --domain=$NGROK_DOMAIN --log=stdout"
        info "Starting ngrok with reserved domain: $NGROK_DOMAIN"
    else
        info "Starting ngrok (ephemeral URL)..."
    fi

    # shellcheck disable=SC2086
    $NGROK_BIN $ngrok_args > /tmp/ngrok.log 2>&1 &
    sleep 3
    return 0
}

get_ngrok_url() {
    curl -s http://localhost:4040/api/tunnels 2>/dev/null \
        | grep -o '"public_url":"https://[^"]*"' \
        | head -1 \
        | cut -d'"' -f4
}

update_env_webhooks() {
    local ngrok_url="$1"
    [ -z "$ngrok_url" ] && return

    # Update BASE_URL
    if grep -q "^BASE_URL=" "$REPO_DIR/.env"; then
        sed -i "s|^BASE_URL=.*|BASE_URL=$ngrok_url|" "$REPO_DIR/.env"
    else
        echo "BASE_URL=$ngrok_url" >> "$REPO_DIR/.env"
    fi

    # Write webhook URLs
    for provider in plaid teller akahu; do
        local upper_provider
        upper_provider=$(echo "$provider" | tr '[:lower:]' '[:upper:]')
        local webhook_key="${upper_provider}_WEBHOOK_URL"
        local endpoint="/api/$provider/webhook"

        if grep -q "^${webhook_key}=" "$REPO_DIR/.env"; then
            sed -i "s|^${webhook_key}=.*|${webhook_key}=$ngrok_url$endpoint|" "$REPO_DIR/.env"
        else
            echo "${webhook_key}=$ngrok_url$endpoint" >> "$REPO_DIR/.env"
        fi
    done

    # Plaid redirect URI
    if grep -q "^PLAID_REDIRECT_URI=" "$REPO_DIR/.env"; then
        sed -i "s|^PLAID_REDIRECT_URI=.*|PLAID_REDIRECT_URI=$ngrok_url/connections/callback/plaid|" "$REPO_DIR/.env"
    fi
}

# ---- build ----
info "Building server binary..."
CGO_ENABLED=0 go build -o /tmp/probably-server ./cmd/server 2>&1
success "Server built"

# ---- ngrok ----
NGROK_URL=""
if start_ngrok; then
    NGROK_URL=$(get_ngrok_url)
    if [ -n "$NGROK_URL" ]; then
        success "ngrok tunnel: $NGROK_URL"
        update_env_webhooks "$NGROK_URL"
    else
        warn "ngrok started but URL not yet available — check /tmp/ngrok.log"
    fi
fi

# ---- migrate ----
info "Running database migrations..."
if [ -n "$DATABASE_URL" ]; then
    DATABASE_URL="$DATABASE_URL" go run ./cmd/util migrate
else
    go run ./cmd/util migrate
fi
success "Migrations complete"

# ---- seed ----
info "Seeding dev user..."
if [ -n "$DATABASE_URL" ]; then
    DATABASE_URL="$DATABASE_URL" go run ./cmd/util seed
else
    go run ./cmd/util seed
fi

# ---- start server ----
info "Starting application server on port $PORT..."
pkill -f "probably-server" 2>/dev/null || true
sleep 1

# Re-source .env so the server picks up any ngrok URL updates
set -a
source "$REPO_DIR/.env" 2>/dev/null || true
set +a

RUNTIME_MODE=app /tmp/probably-server > /tmp/probably-server.log 2>&1 &
SERVER_PID=$!

# Wait for server to respond
MAX_WAIT=60; WAITED=0
while [ "$WAITED" -lt "$MAX_WAIT" ]; do
    if curl -sf "http://localhost:$PORT" > /dev/null 2>&1; then
        break
    fi
    WAITED=$((WAITED + 1))
    sleep 1
done

if curl -sf "http://localhost:$PORT" > /dev/null 2>&1; then
    success "Server is ready"
else
    warn "Server did not respond within ${MAX_WAIT}s — check /tmp/probably-server.log"
fi

# ---- summary ----
LOCAL_URL="http://localhost:$PORT"
PUBLIC_URL="${NGROK_URL:-$LOCAL_URL}"

echo ""
echo -e "${BOLD}============================================================${NC}"
echo -e "${GREEN}  Probably cloud environment is ready!${NC}"
echo -e "${BOLD}============================================================${NC}"
echo ""
echo -e "  ${BOLD}App URL:${NC}      $PUBLIC_URL"
echo -e "  ${BOLD}Local URL:${NC}    $LOCAL_URL"
echo ""
echo -e "  ${BOLD}Dev login:${NC}"
echo -e "    Email:    $DEV_EMAIL"
echo -e "    Password: $DEV_PASSWORD"
echo ""
if [ -n "$NGROK_URL" ]; then
echo -e "  ${BOLD}Webhook URLs:${NC}"
echo -e "    Plaid:    $NGROK_URL/api/plaid/webhook"
echo -e "    Teller:   $NGROK_URL/api/teller/webhook"
echo -e "    Akahu:    $NGROK_URL/api/akahu/webhook"
echo -e "    Stripe:   $NGROK_URL/api/stripe/webhook"
echo ""
echo -e "  ${BOLD}ngrok dashboard:${NC} http://localhost:4040"
fi
echo -e "  ${BOLD}Database:${NC}     ${DATABASE_URL:-<not set>}"
echo -e "  ${BOLD}Server logs:${NC}  /tmp/probably-server.log"
echo -e "  ${BOLD}ngrok logs:${NC}   /tmp/ngrok.log"
echo ""
echo -e "  Server PID: $SERVER_PID"
echo -e "${BOLD}============================================================${NC}"
