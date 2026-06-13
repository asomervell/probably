# Connect Claude to Probably

Connect Claude Code (CLI) or Claude Desktop to your Probably MCP server so you
can ask Claude about your real finances. This takes about two minutes and no API
key.

## What you'll have when you're done

Claude connected to your Probably account, able to answer questions like
**"How much did I spend on groceries last month?"** using your actual data.

## Prerequisites

- **Claude Code CLI** _or_ **Claude Desktop** installed.
- A **Probably account** (you'll sign in during the OAuth step below).
- Your **Probably MCP base URL** (for example `https://app.probably.fyi`). If you
  aren't sure, fetch the live config — it always reflects the running server:

  ```bash
  curl -s https://YOUR_PROBABLY_HOST/.well-known/mcp-client-config
  ```

  Everywhere below, replace `<baseURL>` with your base URL and append `/mcp` as
  shown. A repo-shipped fallback copy lives at
  [`client-config.sample.json`](./client-config.sample.json).

## Option A: Claude Code (CLI)

Run one command:

```bash
claude mcp add probably <baseURL>/mcp
```

Expected output is a confirmation that the `probably` server was added, e.g.:

```
Added MCP server "probably" (<baseURL>/mcp)
```

On your **first tool call**, Claude Code opens a browser to complete the Probably
OAuth sign-in (see [First sign-in](#first-sign-in-oauth) below). After that,
you're connected.

## Option B: Claude Desktop (config JSON)

1. Fetch the config block from the live endpoint:

   ```bash
   curl -s <baseURL>/.well-known/mcp-client-config
   ```

2. Copy the `claude_desktop.mcpServers.probably` block into your Claude Desktop
   config file:

   - **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
   - **Windows:** `%APPDATA%/Claude/claude_desktop_config.json`

   It should look like this (merge it into any existing `mcpServers`):

   ```json
   {
     "mcpServers": {
       "probably": {
         "url": "<baseURL>/mcp"
       }
     }
   }
   ```

3. **Restart Claude Desktop** so it picks up the new server.

If the live endpoint is unreachable, use the repo-shipped fallback
[`client-config.sample.json`](./client-config.sample.json) and replace
`YOUR_PROBABLY_HOST` with your host.

## First sign-in (OAuth)

The first time Claude calls a Probably tool, it opens your browser to sign in to
Probably. **No API key paste is required** — it's a standard OAuth flow.

Probably requests exactly these four read-only scopes:

- `read:transactions`
- `read:accounts`
- `read:financial`
- `read:patterns`

Approve them and the browser hands you back to Claude. The session is reused for
later calls.

## Verify it works

Ask Claude:

> **How much did I spend on groceries last month?**

You should see Claude make a Probably tool call and then answer with a figure
sourced from your real Probably data (not a guess). That round-trip confirms the
connection is working end to end.

## Troubleshooting

- **URL must end in `/mcp`.** `<baseURL>/mcp`, not `<baseURL>` alone.
- **Browser didn't open?** Visit `<baseURL>/mcp/auth` manually to start the OAuth
  sign-in.
- **Seeing `unauthorized`?** Your session expired — re-add the server
  (`claude mcp add probably <baseURL>/mcp`) or reconnect in Claude Desktop.
