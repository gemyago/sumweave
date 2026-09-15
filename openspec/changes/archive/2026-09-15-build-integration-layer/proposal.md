## Why

Sumweave's browser-session API already exposes the finance reads and asynchronous
operations needed by external workflows, but scripts and coding agents have no
long-lived, revocable credential or supported direct client. Phase 1 should make
that existing surface safely usable without creating a second integration API or
broadening the finance product boundary.

## What Changes

- Add self-service personal access tokens owned by existing auth users, with one
  `read-only` or `read-write` permission, optional expiry, one-time secret
  display, immediate rotation/revocation, and digest-only PostgreSQL storage.
- Make bearer authentication credential-aware and decorate every protected
  application operation with an explicit session-only, token-read, or token-write
  policy; new operations remain session-only by default.
- Allow tokens to use only the canonical current-user, tenant, account,
  transaction, current provider-snapshot, connection, selected finance-trigger,
  and requester-scoped jobs operations on their existing routes and contracts.
- Add token- or session-user-scoped `Idempotency-Key` support to bank sync,
  classification, and transfer-matching submissions while preserving
  appdispatch-first execution and lazy observed-job materialization.
- Scope browser and token job reads to the caller and permitted requester
  sources, add the `integration` requester source, remove worker identity from
  the user-facing job detail, and return the shared safe JSON error envelope.
- Add the protected self-service `#/admin/access-tokens` screen for creating,
  listing, rotating, revoking, copying, and then discarding one-time secrets.
- Add the separate Go `swmd` HTTP client entrypoint with persisted credential
  configuration through required `--base-url ... --token-stdin`, JSON output,
  all token-allowed resource commands, and bounded observed-job waiting.
- **BREAKING** Change the existing bank-sync trigger success status from `200`
  to `202`, bound transaction list pages to 1–200 with a default of 100, scope
  jobs reads to the signed-in requester, remove `workerId` from job detail, and
  standardize app-route errors on `code`, `message`, and `correlationId`.

## Capabilities

### New Capabilities

- `personal-access-tokens`: Self-service token lifecycle, secure credential
  validation, explicit operation policies, current-caller discovery, and safe
  shared API errors.
- `direct-finance-cli`: Local `swmd` configuration, token-authenticated commands
  over the existing API, JSON/diagnostic output behavior, transport safety, and
  bounded job waiting.

### Modified Capabilities

- `finance-management`: Allowlist existing finance reads and three asynchronous
  triggers for token callers, bound transaction pagination, derive integration
  requester metadata, and support retry-safe trigger publication.
- `durable-jobs`: Restrict list/detail reads by caller and requester source,
  support integration-requested projections, and keep worker identity internal.
- `finance-operator-ui`: Add the protected self-service Access tokens Admin
  route and its one-time-secret lifecycle.
- `database-migration-command`: Prepare the new app-owned access-token table in
  the explicit PostgreSQL migration root.

## Impact

- Backend app: `apps/sumweave/internal/auth`, HTTP middleware/controllers,
  `v1routes.yaml` and generated routes/models, app errors, jobs reads,
  appdispatch error mapping, explicit HTTP/migration wireup, and focused tests.
- Finance module: bank-sync, classification, and transfer-matching submission
  parameters and requester/idempotency propagation only; existing execution,
  tenant membership, and provider-snapshot boundaries remain unchanged.
- Direct client: new `apps/sumweave/cmd/swmd` entrypoint and
  `apps/sumweave/internal/swmdclient` package; no server, persistence, runtime,
  appdispatch, or jobs-store imports.
- UI: Admin route, subnavigation/overview, auth client models/calls, responsive
  token-management screen, tests, and wireframe documentation.
- Persistence: one app-owned `auth_access_tokens` table; no finance,
  appdispatch, jobs, or runtime schema change and no compatibility migration.
- Operations: existing PostgreSQL bootstrap and split API/worker processes are
  retained; release packaging, installers, SDKs, OAuth, webhooks, and runtime
  agent token access remain out of scope.
