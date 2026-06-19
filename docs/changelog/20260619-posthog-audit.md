# 2026-06-19 — PostHog audit session

## Prior context loaded

Read `20260619.md` (earlier session today). Key facts:
- All PostHog error-tracking issues: resolved/suppressed — 0 active
- Exception events: 0 in last 7 days (and 0 in last 30 days)
- App telemetry silent for 30+ days (no OTEL data reaching PostHog)
- Last code HEAD: `fb2bbf9` (docs: 2026-06-19 memory log)
- No open PRs

## What I queried

1. PostHog exceptions — last 7 days → **0 results**
2. PostHog exceptions — last 30 days → **0 results**
3. PostHog exceptions — last 90 days → **923 results**, all on **2026-04-25**

## Historical errors analysed (all 2026-04-25, 55+ days ago)

### Error A — Gemini `function_response.name: Name cannot be empty` (~894 events)

Gemini API HTTP 400. Tool-response messages were sent with an empty `Name` field.

Root cause: The `convertMessagesToVertexContents` builder didn't always have a fallback when `ToolCallID` was empty and the `expectedNames` queue ran dry. Additionally, on the OpenAI-compat (non-Vertex) Google path, thinking models returned function calls even with no tool definitions, corrupting the message history.

**Already fixed in codebase:**
- `sanitizeFunctionResponseNames()` (api.go:1161) — final-pass sanitiser sets empty names to `"search_entities"`.
- `allowToolCalls = false` guard (simple.go:71) — disables tool calling on the Google non-Vertex path to prevent orphaned function calls.
- `resolveCategorizeToolName` / `annotateToolResponse` (simple.go:291,309) — multi-level name fallback before tool responses are appended to the message history.

### Error B — Gemini `thought_signature missing` (~228 events)

Gemini API HTTP 400. Some FunctionCall parts had thought signatures, others did not — Gemini rejects mixed-signature batches.

**Already fixed in codebase:**
- `thoughtSignaturesConsistent()` (api.go:1276) — only preserves `RawVertexParts` when ALL or NONE of the function-call parts carry signatures.
- `thinkingConfig()` (api.go:1262) — disables thinking (budget=0) whenever tool history exists.

### Error C — PostHog OTEL `Too many AI spans` (~20 events)

PostHog `/i/v0/ai/otel` endpoint rejected batches with >100 AI spans.

**Already fixed in codebase:**
- `WithMaxExportBatchSize(50)` + `WithMaxQueueSize(75)` in `InitOTEL` (otel.go:79-81) — caps each export batch well under the 100-span limit.

### Error D — HTTP 500 `GET /connections/connect/plaid` (9 events, 2026-04-25)

Server returned 500 when `createLinkTokenWithRedirectFallback` failed or when the user's person entity wasn't found.

This is operational (Plaid unconfigured / user account incomplete) rather than a code defect. The handler already returns an appropriate 500 with a descriptive message. Not actionable as a code change.

## Reasoning

All errors are from April 25, 2026. The current codebase (initial public commit June 13 + Claude sessions) already contains fixes for Errors A, B, and C. Error D is operational. Zero active errors exist.

Creating PRs for these would manufacture fake work. Per standing instructions: **0 active errors → write changelog, stop.**

## Shipped this session

Nothing — no new code issues found.

## What was left behind

- App telemetry silence: 0 OTEL events for 55+ days. Likely a deployment/config gap (PostHog project key not passed to prod binary). Not actionable here.
- 5 Akahu connections with expired OAuth tokens — operational, user must reconnect via app UI.

## Next run checklist

1. **READ THIS FILE FIRST** (or `20260619.md`) — `asomervell/probably`, 2026-06-19.
2. `git fetch origin main && git log --oneline origin/main -5`. Last known HEAD: `fb2bbf9`.
3. Query PostHog exceptions last 7 days → expect 0. All known errors resolved.
4. Query PostHog logs (warn/error) last 7 days → expect 0.
5. If telemetry starts flowing again → look at `partial_failure` pattern and new error types.
6. If 0 active errors AND 0 new code issues → write changelog, stop. Do NOT manufacture work.
7. PostHog assignee API: HTTP 500 — do not attempt programmatic assignment.
8. Auto-merge not enabled at repo level — cannot enable via API.
9. No CI workflows on this repo.
10. Fixed historical errors (A/B/C above) are already in the codebase — do not re-investigate.
