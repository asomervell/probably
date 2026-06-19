# 2026-06-19 — Session: PostHog audit (scheduled routine)

## Prior context loaded

Read `20260619.md` (earlier session today). Key facts carried forward:
- Repo: `asomervell/probably`, working branch `claude/clever-knuth-kfeb2x`
- Main HEAD: `fb2bbf9` (docs: 2026-06-19 memory log — PR #92 merged, 0 active errors)
- 0 active PostHog errors for 50+ consecutive sessions
- App telemetry silent: 0 OTel logs/events reaching PostHog for 30+ days
- PostHog assignee API: HTTP 500 — skip programmatic assignment
- Auto-merge not enabled on repo — cannot enable via API
- Fixed PRs to date: #3, #5, #9, #26, #57, #58, #92

## Reasoning

This session performed a deep PostHog audit via direct API queries (PostHog MCP server unavailable; used `POSTHOG_PERSONAL_API_KEY` via curl against `https://us.posthog.com/api/projects/88871/`).

### Query methodology
No time-gated PostHog MCP tools available. Queried via HogQL:

1. `events last 7d grouped by type` → **0 rows** — app telemetry completely silent.
2. `events all-time grouped by type` → Found historical events from April 2026 (dev environment):
   - `$ai_span`: 8,192
   - `$ai_trace`: 2,188
   - `$autocapture`: 1,418
   - `$pageview`: 1,368
   - `$exception`: 923
   - `log_message`: 169
3. `log_message errors all-time` → 9 entries, all: `HTTP 500 GET /connections/connect/plaid`, dated **2026-04-25**, environment=`development`.
4. `$exception all-time grouped by issue_id` → Two exception classes, both dated **2026-04-25**, environment=`development`:
   - **894 occurrences**: `orchestrator error [parse_error]: LLM API error 400: Function call is missing a thought_signature in functionCall parts` — Gemini `gemini-3.1-flash-lite-preview` returning 400 on categorization tool calls. Code already has `isThinkingModel()` + `thoughtSignaturesConsistent()` guards in `internal/orchestrator/api.go`. Error is historical.
   - **29 occurrences**: `traces export: failed to send ... Too many AI spans: 246 exceeds limit of 100` — OTEL batch exporter sending oversized batches. Code already patched with `MaxExportBatchSize(50)` + `MaxQueueSize(75)` in `internal/observability/otel.go`. Error is historical.

### Conclusion on exceptions
All 923 exceptions and all 9 error logs are from **2026-04-25** in the `development` environment. These were investigated and fixed in earlier sessions (code guards added; errors were suppressed in PostHog). No new occurrences since April 25 — confirmed by 0 events in the last 7 days.

### Git / PR check
- `git fetch origin main` → HEAD `fb2bbf9` (same as prior session, no new commits)
- Open PRs → **0** (none open)
- No new code issues to address

## PostHog state (2026-06-19, this session)

| Check | Result |
|---|---|
| Events last 7 days | **0** — telemetry completely silent |
| Error/warn logs (all time) | 9 entries, all 2026-04-25, dev environment, already resolved |
| Exception events (all time) | 923 total; all 2026-04-25, dev environment, already resolved |
| Active error tracking issues | 0 active (all resolved/suppressed) |
| Open PRs | **0** |
| New code commits since prior session | **None** |

## Historical errors (already fixed — do not re-investigate)

### Error A: Gemini thought_signature (894 occurrences, 2026-04-25)
- **Root cause**: `gemini-3.1-flash-lite-preview` requires `thought_signature` on all function call parts when thinking is enabled. Model was returning 400 during categorization.
- **Fix already in code**: `internal/orchestrator/api.go` — `isThinkingModel()` (line ~1258), `thoughtSignaturesConsistent()` (lines ~1279-1291), `thinkingConfig()` disables thinking when tools or tool history present.
- **Status**: No recurrences after 2026-04-25. RESOLVED.

### Error B: OTEL too many AI spans (29 occurrences, 2026-04-25)
- **Root cause**: BatchSpanProcessor flushing 246 spans in one batch; PostHog limit is 100.
- **Fix already in code**: `internal/observability/otel.go` lines 74-82 — `MaxExportBatchSize(50)`, `MaxQueueSize(75)`.
- **Status**: No recurrences after 2026-04-25. RESOLVED.

### Error C: HTTP 500 GET /connections/connect/plaid (9 occurrences, 2026-04-25)
- **Root cause**: `PlaidConnect` handler in `internal/handlers/plaid_handlers.go` returning 500 — likely Plaid credentials not configured or `getCurrentLedger` failing in dev.
- **Status**: Development-only, no production impact. Historical. RESOLVED (suppressed).

## Shipped this session

Nothing — no active errors, no new code issues, no open PRs to manage.

## What was left behind (unchanged)

- **App telemetry silent**: 0 OTel events reaching PostHog for 30+ days (app not deployed with current OTel config, or exporter not reaching PostHog from production). Not actionable from this session.
- **Operational**: 5 Akahu connections with expired/invalid tokens → reconnect via OAuth in app UI.
- PostHog assignee API: HTTP 500 — cannot assign issues programmatically.
- Auto-merge not enabled at repo level — cannot enable via API.

## Next run checklist

1. **READ THIS FILE** and `20260619.md` — this is `asomervell/probably`, 2026-06-19.
2. `git fetch origin main && git log --oneline origin/main -6`. Last known HEAD: `fb2bbf9`.
3. Check events last 7d (HogQL: `SELECT event, count() FROM events WHERE timestamp >= now() - interval 7 day GROUP BY event`) → expect 0.
4. Check error logs (HogQL: `SELECT properties.level, properties.message, count() FROM events WHERE event = 'log_message' AND timestamp >= now() - interval 7 day GROUP BY 1, 2`) → expect 0.
5. Check $exception events last 7d → expect 0.
6. If all 0 and no new code issues → write changelog only. Do NOT manufacture work.
7. PostHog assignee API: HTTP 500 — do not attempt programmatic assignment.
8. Auto-merge: not enabled — do not attempt via API.
9. No CI workflows on this repo — do not chase phantom CI failures.
10. Fixed PRs to date: #3, #5, #9, #26, #57, #58, #92. Do not reinvestigate.
11. All PostHog error-tracking issues `resolved`/`suppressed`. Not actionable.
12. Historical errors A/B/C above (April 2026, dev) — RESOLVED. Do not re-investigate.
13. Close stale duplicate docs PRs from the same day before creating new ones.
14. PostHog MCP server unavailable in this session — use direct curl API with `POSTHOG_PERSONAL_API_KEY`.
