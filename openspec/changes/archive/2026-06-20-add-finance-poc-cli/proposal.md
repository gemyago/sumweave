## Why

Issue #26 asks for a lightweight proof-of-concept that stretches Signal Foundry from a trading assistant toward a broader financial assistant. The immediate need is not a productized finance domain, UI, persistence model, or consent platform; it is a small CLI surface that can prove account and transaction access against Enable Banking/PKO and monobank while respecting repo security rules.

## What Changes

- Add an isolated `finance-poc` Cobra command tree under the existing `apps/signal-foundry/cmd/signal-foundry` binary.
- Add direct HTTP POC clients for Enable Banking and monobank using env-var/flag configuration, context timeouts, fake-server-testable base URLs, raw JSON preservation, and thin normalized summaries.
- Add Enable Banking commands for Polish ASPSP discovery, explicit `start-auth`/`finish-session` manual authorization, optional local-callback `connect`, session account listing with optional details/balances, and paginated account transactions.
- Fix the Enable Banking `connect` local-callback flow so the generated localhost callback URL uses HTTPS, using paired operator-supplied trusted certificate/key files when both are provided, failing clearly when only one is provided, and using an ephemeral self-signed fallback only when neither is provided.
- Add monobank commands for client-info account/jar/managed-client listing and date-chunked statement transaction fetching.
- Add documentation under `apps/signal-foundry/doc/financial-poc/` for manual provider setup, live command examples, API limits, restricted production account linking, and AI-agent test constraints.
- Confirm or add ignore rules that prevent private keys, tokens, session files, transaction exports, and live bank output from being committed.
- Add offline tests with `httptest.Server`; live provider credentials remain optional and manually supplied only.

## Capabilities

### New Capabilities

- `financial-poc-cli`: Lightweight CLI proof-of-concept for Enable Banking/PKO and monobank account and transaction access.

## Impact

- Affects `apps/signal-foundry/cmd/signal-foundry` command wiring and focused command/client tests.
- Affects `apps/signal-foundry/doc/financial-poc/` documentation.
- Affects Enable Banking `connect` callback guidance and tests for local HTTPS listener behavior.
- May affect root `.gitignore` only to add explicit financial POC secret/output safeguards if existing `/data` coverage is not sufficient for all required file patterns.
- Does not affect `runtime/`, `apps/signal-ui`, app HTTP APIs, product database schemas, OpenAPI routes, DI wiring, or AI assistant tools.
