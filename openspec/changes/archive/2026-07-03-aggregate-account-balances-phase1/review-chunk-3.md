# Review Chunk 3

## Implementation Round 1 — 2026-07-03

- Implementer: openspec-implementation
- Scope: task `3.1`
- Status: complete

### OpenSpec apply

- Attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change aggregate-account-balances-phase1 --task 3`.
- Installed CLI still does not expose `apply` and returned `unknown command 'apply'`.
- Used `openspec instructions tasks --change aggregate-account-balances-phase1` for task context and stayed within chunk `3.1` scope.

### Verification work completed

- Re-ran implementation verification checks with `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/...` and `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`.
- Ran the design-mandated local backend flow with fresh migration plus PM2 restart/status confirmation before API checks.
- Authenticated with the first repo-root `.local-users` entry and created a fresh tenant `agg-balance-phase1-1783100006`.
- Created three manual USD accounts:
  - checking: `d6f7cf84-d796-4a57-8642-9cc2b1b3d36a`
  - savings: `eb69fd61-cd92-4712-b8c2-dd9790c4a2e2`
  - cash: `d100d0c6-5361-4c7a-b3c9-f5eee32d8aa8`
- Recorded mixed transactions through the API:
  - checking opening balance `+100000` booked
  - checking regular income `+250000` booked
  - checking regular expense `-40000` booked
  - checking refund `+10000` booked
  - checking transfer out `-30000` booked
  - savings transfer in `+30000` booked
  - savings reconciliation `+5000` booked
  - checking regular expense `-12000` pending
- Verified `GET /api/v1/finance/tenants/fb99e55e-c237-4bbf-a980-96c46c8811f6/accounts` returned expected balances:
  - checking `bookedBalanceMinor=290000`, `pendingBalanceMinor=-12000`
  - savings `bookedBalanceMinor=35000`, `pendingBalanceMinor=0`
  - cash `bookedBalanceMinor=0`, `pendingBalanceMinor=0`
- Verified `GET /api/v1/finance/tenants/fb99e55e-c237-4bbf-a980-96c46c8811f6/accounts/d6f7cf84-d796-4a57-8642-9cc2b1b3d36a` and `.../accounts/eb69fd61-cd92-4712-b8c2-dd9790c4a2e2` returned the same expected balance values.
- No mismatches were found, so no follow-up code fix was required in this chunk.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/...`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`
- `direnv exec /Users/jenya/projects/signal-foundry/apps/signal-foundry go run ./cmd/signal-foundry db-migrate --env local`
- `direnv exec /Users/jenya/projects/signal-foundry pm2 restart signal-foundry-api --update-env`
- `direnv exec /Users/jenya/projects/signal-foundry pm2 status`
- Manual API-level verification flow against `http://127.0.0.1:4501`

### OpenSpec task updates

- Marked `tasks.md` item `3.1` complete.

### Artifact cleanup

- Clean.
- No ad-hoc repository artifacts added.
- Created the standard chunk artifact `review-chunk-3.md` because it was referenced in `manager-status.md` but missing before this run.

### Follow-up notes for reviewer

- Manual verification passed on the PM2-backed local API with the explicit booked/pending math from the design.
- The created verification tenant can be archived or reused later if someone wants to inspect the exact API-written ledger state.

## Review Round 2 — 2026-07-03

- Scope reviewed: task `3.1`
- Reviewer: openspec-implementation-finalizing
- Verdict: clean
- Complete to continue: yes
- Result: `complete`

### Findings

- None. Manual API checks match the design flow and expected math.

### Completion protocol check

- `make affected-lint-test` was already run during chunk implementation and reported no errors.
- Completion protocol at the repo level is therefore satisfied for this chunk.

### OpenSpec task and protocol checks

- `openspec apply` could not be run because the installed CLI does not expose an `apply` subcommand.
- `tasks.md` item `3.1` is marked `[x]`.

### Artifact cleanup status

- Clean: only standard OpenSpec artifacts were added or updated (`review-chunk-3.md`, `tasks.md`, `manager-status.md`).

### Commit status

- Committed in `090dc09`.

### Gate decision

- Safe to continue: yes.
- Continue target: user review/correction phase.

### Follow-up chunks

- none

## Implementation Round 3 — 2026-07-03

- Implementer: openspec-implementation
- Scope: doc correction for manual API verification guide under task `3.1`
- Status: complete

### Summary

- Revised `docs/manual-e2e/finance-account-balances-e2e.md` to match current finance API conventions.
- Updated account create examples to use `{"name":"...","kind":"manual","currency":"USD"}` bodies.
- Updated transaction examples to use snake_case kinds: `opening_balance`, `regular`, `refund`, `transfer`, and `reconciliation`.
- Replaced transaction timestamp field `occurredAt` with `effectiveAt` in all request bodies.
- Preserved the same API-only flow, request sequencing, and expected booked/pending balance math.
- Left `docs/manual-e2e/README.md` unchanged so it still links to the guide.

### Checks run

- None. Documentation-only correction.

### OpenSpec task updates

- None. Existing task `3.1` remained complete; this run only aligned the published runbook with API conventions.

### Artifact cleanup

- Clean.
- No ad-hoc repository artifacts added.
