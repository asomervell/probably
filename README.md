# Probably

A simple, self-hosted personal finance application with double-entry bookkeeping.

## Install

```bash
curl -fsSL https://probably.money/install.sh | sh
```

Then run:

```bash
probably
```

Open http://localhost:8080 — that's it!

<details>
<summary><b>Manual installation</b></summary>

**Prerequisites:** Go 1.21+ ([download](https://go.dev/dl/))

```bash
git clone https://github.com/asomervell/probably.git
cd probably
docker compose up db -d
export DATABASE_URL="postgres://finance:secret@localhost:5432/finance?sslmode=disable"
go run ./cmd/server
```

</details>

### What happens on first run

- Migrations run against the database in `DATABASE_URL`
- Create `.env` from 1Password (`make env`) for keys and a standard `DATABASE_URL`, or set `DATABASE_URL` yourself

## Features

- **Double-Entry Ledger**: Proper accounting with balanced transactions
- **Bank Syncing**: Connect to US banks via Teller.io
- **Smart Categorization**: Rule-based and AI-powered (xAI Grok) transaction tagging
- **Transfer Detection**: Automatic matching of inter-account transfers
- **Multi-User Support**: Households can share or have separate ledgers
- **Beautiful UI**: Modern dark theme with Tailwind CSS

## Connecting Claude

Connect Claude Code or Claude Desktop to Probably's MCP server and ask about your
real finances. See the step-by-step guide:
[docs/claude-code/README.md](docs/claude-code/README.md).

## Tech Stack

- **Backend**: Go with Chi router
- **Database**: PostgreSQL (you supply `DATABASE_URL`, e.g. Docker Compose in this repo)
- **Frontend**: Gomponents (Go HTML) + Tailwind CSS + HTMX
- **Auth**: Authboss (full-featured auth library)
- **Bank Sync**: Teller.io API
- **AI**: xAI Grok (with function calling)

---

## Running Modes

### 1. PostgreSQL required

`DATABASE_URL` must point at a running PostgreSQL instance (local, Docker, or hosted). Example with Docker from this repo:

```bash
docker compose up db -d
export DATABASE_URL="postgres://finance:secret@localhost:5432/finance?sslmode=disable"
go run ./cmd/server
```

### 2. Development with Hot Reload

```bash
# Install air (one-time)
go install github.com/air-verse/air@latest

# Run with hot reload
air
```

### 3. Desktop App (Native Window)

```bash
# Install Wails CLI (one-time)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Run desktop app
wails dev
```

---

## Configuration

`DATABASE_URL` is required. Create a `.env` file (for example with 1Password) to supply it and other secrets:

```bash
make env   # injects secrets from 1Password (requires op CLI + OP_SERVICE_ACCOUNT_TOKEN or personal account)
```

### Core Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | *(required, no default)* |
| `SESSION_SECRET` | 32+ char secret for sessions | Auto-generated for dev |

### Teller.io (Bank Syncing)

| Variable | Description |
|----------|-------------|
| `TELLER_APP_ID` | Your Teller.io application ID |
| `TELLER_ENVIRONMENT` | `sandbox` or `production` |
| `TELLER_CERT` | Base64-encoded Teller certificate |
| `TELLER_KEY` | Base64-encoded Teller private key |
| `TELLER_WEBHOOK_SECRET` | Webhook signing secret |

### xAI Grok (Smart Categorization)

| Variable | Description |
|----------|-------------|
| `XAI_API_KEY` | xAI API key (primary) |
| `GROK_MODEL` | Model to use (default: `grok-3`) |
| `GROQ_API_KEY` | Groq API key (fallback if xAI not set) |
| `GROQ_MODEL` | Groq model (default: `llama-3.3-70b-versatile`) |

---

## Data Storage Locations

| Platform | Location |
|----------|----------|
| macOS | `~/Library/Application Support/Probably/` |
| Linux | `~/.local/share/probably/` |
| Windows | `%APPDATA%\Probably\` |

To reset all data, delete this folder and restart.

---

## Account Types

The double-entry ledger uses standard accounting types:

- **Asset**: Bank accounts, investments, cash (debits increase balance)
- **Liability**: Credit cards, loans (credits increase balance)
- **Income**: Salary, dividends (credits increase)
- **Expense**: Purchases, bills (debits increase)
- **Equity**: Opening balances, adjustments

## How Transactions Work

Every transaction has two or more entries that must sum to zero:

```
# Buying groceries with debit card
Debit:  Groceries (Expense)     +$50.00
Credit: Checking (Asset)        -$50.00
                                -------
                                 $0.00

# Credit card payment
Debit:  Credit Card (Liability) +$500.00  (reduces debt)
Credit: Checking (Asset)        -$500.00  (reduces balance)
```

---

## Setting Up Bank Sync (Teller.io)

Teller.io provides secure connections to US banks. You need a free developer account.

### Quick Setup (Recommended)

Run the interactive setup script:

```bash
./scripts/setup-teller.sh
```

This will guide you through the entire process.

### Manual Setup

<details>
<summary>Click to expand manual instructions</summary>

#### 1. Create a Teller Account

1. Go to [teller.io](https://teller.io) and click "Get API Keys"
2. Create your account and verify your email
3. Access your [Dashboard](https://teller.io/dashboard)

#### 2. Get Your Credentials

From your Teller dashboard:

- **Application ID**: Displayed at the top (starts with `app_`)
- **Certificates**: Go to "Certificates" → "Generate Certificate"
  - Download `certificate.pem`
  - Download `private_key.pem`

#### 3. Store Certificates Securely

Place certificate files in the `.secure/` directory (git-ignored):

```bash
mkdir -p .secure
mv ~/Downloads/certificate.pem .secure/
mv ~/Downloads/private_key.pem .secure/
```

#### 4. Configure Environment

Base64 encode your certificates and add to `.env`:

```bash
# Create .env if it doesn't exist
make env

# Encode certificates (macOS/Linux)
TELLER_CERT=$(base64 < .secure/certificate.pem | tr -d '\n')
TELLER_KEY=$(base64 < .secure/private_key.pem | tr -d '\n')

# Add to .env (or edit manually)
echo "TELLER_APP_ID=app_your_app_id" >> .env
echo "TELLER_ENVIRONMENT=sandbox" >> .env  
echo "TELLER_CERT=$TELLER_CERT" >> .env
echo "TELLER_KEY=$TELLER_KEY" >> .env
```

</details>

### Environments

| Environment | Description |
|-------------|-------------|
| `sandbox` | Test with fake banks. Username: `username`, Password: `password` |
| `production` | Connect to real US banks |

### Webhooks (Optional)

For real-time transaction updates:

1. In Teller Dashboard → Application Settings
2. Set webhook URL: `https://yourdomain.com/api/teller/webhook`
3. Copy the signing secret to `TELLER_WEBHOOK_SECRET` in `.env`

---

## Setting Up AI Categorization (xAI Grok)

1. Sign up at [console.x.ai](https://console.x.ai)
2. Create an API key
3. Add to your `.env`:
   ```
   XAI_API_KEY=xai-xxx
   ```

Transactions are automatically categorized in the background via a rate-limited worker. xAI Grok supports function calling, which enables merchant search to reduce duplicate merchant records.

---

## Project Structure

```
probably/
├── cmd/
│   └── server/          # Web server entry point
├── internal/
│   ├── auth/            # Authboss configuration
│   ├── categorize/      # AI categorization worker
│   ├── config/          # Configuration loading
│   ├── db/              # Database connection & migrations
│   ├── handlers/        # HTTP handlers
│   ├── ledger/          # Double-entry logic
│   ├── models/          # Data models & stores
│   ├── sync/            # Teller sync & transfer matching
│   └── views/           # Gomponents templates
├── scripts/             # Setup scripts
├── static/              # CSS files
├── main.go              # Desktop app entry point (Wails)
└── wails.json           # Wails configuration
```

---

## Troubleshooting

### Port already in use

```bash
# Use a different port
PORT=3000 go run ./cmd/server
```

### PostgreSQL won't start

Delete the data directory to reset:
- macOS: `rm -rf ~/Library/Application\ Support/Probably/postgres`
- Linux: `rm -rf ~/.local/share/probably/postgres`

### First run is slow

The first run downloads PostgreSQL binaries (~100MB). Subsequent runs start in seconds.

---

## License

MIT
