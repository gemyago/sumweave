# Chunk Review: Enable Banking Typed Contract

## Initial State

- Scope: tasks 1.1-1.2.
- Status: implementation in progress.
- No implementation round has been recorded yet.

## Round 1 — Implementation — 2026-08-14

### Scope

OpenSpec tasks 1.1-1.2 only: establish the complete typed Enable Banking account-information response boundary and stop client normalization from changing provider DTOs.

### Implementation

- Added semantic decode/encode round-trip coverage for the official Enable Banking API reference at `https://enablebanking.com/docs/api/reference/`, checked 2026-08-14. The fixtures cover the session-account response, session account-data response, account details, balances, transaction page, and transaction item.
- Expanded typed provider DTOs for account metadata, clearing member identity, postal addresses, amounts, parties, transaction agents/accounts/codes/references/exchange rate, and optional false/zero/empty values.
- Split `POST /sessions` into its own typed `CreateSessionResponse`; retained `GET /sessions/{id}` as its documented typed response shape.
- Removed DTO convenience/derived fields and all client-side response normalization. Connector mapping now derives normalized finance values from unmodified DTO fields, preserving its existing normalized account, balance, and transaction behavior without adding snapshots (task 3.1 remains pending).
- Updated affected client and connector tests to assert provider DTO fields rather than removed derived fields.

### TDD Evidence

- Before DTO implementation, `go test ./internal/enablebanking/client -run TestDocumentationResponseRoundTrip -count=1` failed because account and transaction fields from the documentation fixtures were dropped during re-encoding.
- After implementation, the same client package test and the focused connector suite passed.

### Checks

- `go test ./internal/enablebanking/client -run TestDocumentationResponseRoundTrip -count=1` — failed first as expected, then passed after implementation.
- `go test ./internal/enablebanking -run TestConnector -count=1` — passed.
- `go test ./internal/enablebanking/client -count=1` — passed.
- `go test ./...` from `finance/` — passed.
- `make lint` from `finance/` — passed with 0 issues.
- `env -u APP_HTTPSERVER_TLS_CERTFILE -u APP_HTTPSERVER_TLS_KEYFILE make affected-lint-test` from the repository root — passed. The initial ordinary command inherited ignored local HTTPS certificate overrides and failed the unrelated Sumweave config test; unsetting those local-only overrides restored its committed-fixture expectation.

### OpenSpec Task Status

- 1.1: complete.
- 1.2: complete.

### Artifact Cleanup

Clean for this chunk. Changes are source/tests/documentation fixtures plus standard OpenSpec manager/review artifacts; no ad-hoc repository artifacts were created.

### Commit Status

No commit created because the user explicitly instructed not to commit.

## Round 2 — Chunk Finalization — 2026-08-14

### Verdict

Safe to continue to `provider-snapshot-domain-persistence`.

### Review

- The implementation is limited to the task 1.1-1.2 typed DTO boundary and the necessary connector normalization consumers. It deliberately does not introduce provider snapshots, persistence, API, UI, or legacy-removal work assigned to later chunks.
- The semantic round-trip test asserts decoded/encoded JSON values rather than bytes and exercises the required nested, continuation, false, zero, and empty cases.
- Provider response models contain neither raw maps nor raw-message/success-body fields; client methods no longer mutate DTOs. Connector normalization now derives its values from those typed fields.
- Applicable repository completion protocol passed. `AGENTS.md` needs no update because commands, workflows, and architecture did not change.

### Continue Decision

Safe to continue. The normal workflow commit gate is intentionally deferred by the user’s explicit no-commit instruction; the manager must preserve that instruction for later chunks.

### Artifact Cleanup

Passed. Only required source/test/fixture and standard workflow files are pending.

### Commit Status

No commit created at user request.
