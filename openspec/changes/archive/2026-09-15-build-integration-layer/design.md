## Context

The accepted Phase 1 product shape is defined by
`docs/integration-layer/integration-layer-prd.md` and
`docs/integration-layer/integration-layer-design.md`, under the product boundary
in `docs/ARCHITECTURE.md`. The application already has the required auth user,
finance, current provider-snapshot, semantic command, appdispatch, observed-job,
Admin UI, and OpenAPI-first HTTP foundations. External callers currently lack a
long-lived credential and direct client, and jobs reads are not requester-scoped.

The implementation spans the app-owned Go API, narrow finance submission
contracts, the Svelte Admin surface, and a second Go command. `finance/` remains
independent from `runtime/`; the generic runtime receives only a session caller's
user identity. PostgreSQL remains the only application database. Existing
finance routes and response models are reused rather than copied into an
integration namespace.

## Goals / Non-Goals

**Goals:**

- Issue and manage revocable personal access tokens without storing recoverable
  token secrets.
- Authenticate tokens as their current owning user and authorize them through a
  closed, reviewable operation allowlist.
- Reuse existing finance and jobs contracts while preserving tenant membership,
  provider-snapshot sanitization, appdispatch-first execution, and lazy job
  observation.
- Make asynchronous trigger retries token- or session-user-scoped and safe.
- Restrict jobs reads to the current user and the caller's permitted requester
  sources.
- Provide a self-service Admin screen and direct `swmd` client for every
  token-allowed operation.
- Preserve shared correlation, safe error, logging, migration, and process
  conventions.

**Non-Goals:**

- OAuth, delegated third-party consent, refresh tokens for integrations, general
  RBAC, per-resource scopes, tenant grants stored on tokens, or cross-user token
  administration.
- Token access to transaction/account mutation, tenant administration,
  connection management, imports, reports, FX administration, categories, tags,
  rules management, manual transfers, runtime agents, or token management.
- A second API namespace, duplicate response models, SDKs, MCP, webhooks, event
  streaming, job cancellation/results/progress, or dead-letter access.
- Literal provider HTTP archives or provider-snapshot history.
- CLI installers, shell completion, package publication, or release-pipeline
  changes.

## Decisions

### Keep one API and attach a closed credential policy to each operation

The app will use one concrete credential middleware constructed in `buildHTTP`.
Controllers consume a narrow interface with:

```go
Require(policy CredentialPolicy, next http.Handler) http.Handler
```

The closed policies are `session-only`, `token-read`, and `token-write`.
Sessions satisfy every policy they currently satisfy; both access-token
permissions satisfy token read; only `read-write` satisfies token write. Every
protected operation explicitly wraps its generated handler at the controller
method. Ordinary authentication defaults to session-only, and policy is never
inferred from method, path, or OpenAPI tags. Public login and refresh remain
unwrapped. Registered-route tests compare executable policy to the full
allowlist.

Token-read operations are exactly:

- `GET /api/v1/auth/me`;
- `GET /api/v1/finance/tenants`;
- `GET /api/v1/finance/tenants/{tenantId}/accounts`;
- `GET /api/v1/finance/tenants/{tenantId}/accounts/{accountId}`;
- account provider-snapshot metadata and detail GETs;
- `GET /api/v1/finance/tenants/{tenantId}/transactions`;
- `GET /api/v1/finance/tenants/{tenantId}/transactions/{transactionId}`;
- transaction provider-snapshot metadata and detail GETs;
- `GET /api/v1/finance/tenants/{tenantId}/connections`;
- `GET /api/v1/jobs` and `GET /api/v1/jobs/{jobId}`.

Token-write operations are exactly:

- `POST /api/v1/finance/tenants/{tenantId}/connections/{connectionId}/sync`;
- `POST /api/v1/finance/tenants/{tenantId}/transactions/classify`;
- `POST /api/v1/finance/tenants/{tenantId}/transactions/match-transfers`.

All other existing and future protected operations remain session-only unless
deliberately added to this list. The OpenAPI document retains one `BearerAuth`
scheme because both credentials use the same wire header; descriptions document
token availability while registered-route tests enforce it.

### Keep app-owned caller context separate from runtime identity

The app-owned auth package will define:

```go
type CredentialKind string

const (
    CredentialKindSession     CredentialKind = "session"
    CredentialKindAccessToken CredentialKind = "access-token"
)

type AccessTokenCaller struct {
    TokenID    string
    TokenName  string
    Permission AccessTokenPermission
    ExpiresAt  *time.Time
}

type Caller struct {
    UserID      string
    Credential  CredentialKind
    AccessToken *AccessTokenCaller
}
```

Credential middleware creates the invariant that `AccessToken` is present only
for an access-token caller and stores `Caller` in app request context. Auth,
finance, and jobs controllers consume that context directly. The runtime route
is wrapped with `session-only` and receives an adapter exposing only `UserID`, so
`runtime/` does not import app credential types or gain token access.

### Use an opaque high-entropy token with digest-only storage

Issued values have this exact form:

```text
swat_<uuid-v7-token-id>_<unpadded-base64url-32-byte-secret>
```

Parsing consumes the reserved prefix and fixed UUID field before reading the
remaining secret; `_` in base64url content is not treated as an unrestricted
delimiter. The service stores only `SHA-256(secret)` as 32 raw bytes and selects
by token ID before constant-time comparison. The complete value is returned
only by create and rotate. Malformed, unknown, mismatched, expired, revoked, and
invalid-permission tokens all return the same `401` response. A bearer starting
with `swat_` is validated only as an access token; every other bearer is
validated only as a session JWT, with no fallback between validators.

Names are trimmed and constrained to 1–100 characters. Permission is the closed
enum `read-only | read-write`. Optional expiry must be later than creation.
Validation queries PostgreSQL on each request; Phase 1 has no token cache.

### Add one app-owned authentication table

`AccessTokenStore.AutoMigrate()` will run after users and refresh tokens in the
explicit `db-migrate` root. The logical table is `auth_access_tokens`, producing
`sumweave_auth_access_tokens` under the default application prefix.

Exact columns are:

- `id UUID PRIMARY KEY`;
- `user_id UUID NOT NULL`;
- `name TEXT NOT NULL`;
- `permission TEXT NOT NULL`;
- `secret_hash BYTEA NOT NULL`;
- `expires_at TIMESTAMPTZ NULL`;
- `revoked_at TIMESTAMPTZ NULL`;
- `created_at TIMESTAMPTZ NOT NULL`;
- `updated_at TIMESTAMPTZ NOT NULL`.

Exact constraints and indexes are:

- `CHECK (char_length(name) BETWEEN 1 AND 100)`;
- `CHECK (permission IN ('read-only', 'read-write'))`;
- `CHECK (octet_length(secret_hash) = 32)`;
- unique partial index `idx_auth_access_tokens_active_user_name` on
  `(user_id, name) WHERE revoked_at IS NULL`;
- index `idx_auth_access_tokens_user_created` on
  `(user_id, created_at DESC, id DESC)`.

There is no foreign key because current auth user IDs are stored in a
text-compatible column while generated values are UUIDv7. The service validates
the user where required and parses its ID before persistence. There is no
plaintext secret, copied tenant ID, per-route scope, refresh token, last-used
timestamp, or soft-delete column. No finance, appdispatch, jobs, or runtime
schema changes are made.

### Keep token lifecycle in one focused service

`AccessTokenService` receives a narrow store, user reader, ID generator, clock,
random reader, and logger through constructor-enforced dependencies.
`AccessTokenStore` owns persistence only.

- Create verifies the current user, inserts one active row, and returns metadata
  plus the one-time token.
- List returns all owned rows newest first and derives `active`, `expired`, or
  `revoked` without exposing the digest.
- Revoke owner-scopes the update and is idempotent for an already revoked owned
  row, preserving the first revocation timestamp.
- Rotate locks one owned active row and, in one transaction, revokes it and
  inserts a replacement retaining name and permission while applying the
  required nullable replacement expiry.
- Missing and non-owned management IDs both return `404`; expired or revoked
  tokens cannot rotate.
- Concurrent rotation attempts serialize on the old row; only one replacement
  is created and later attempts return conflict.

The existing auth controller gains session-only create, list, rotate, and revoke
operations at `/api/v1/auth/access-tokens`. Requests never contain owner IDs.
Create returns `201`, rotate `200`, revoke `204`, and create/rotate share one
issued response with metadata plus `apiToken`. `GET /api/v1/auth/me` adds optional
access-token metadata only for token callers.

### Reuse finance services and preserve their authorization boundary

Finance controllers continue passing `Caller.UserID` as `ActorUserID` into the
existing focused services. Those services continue enforcing current tenant
membership and object ownership. Tenant IDs are never copied onto access-token
rows. Current provider-snapshot list responses remain metadata-only and detail
responses continue the existing pre-persistence and pre-response sanitization.

The shared transaction list gains an optional `limit` default of 100 and maximum
of 200 for every caller; all existing filters, offset pagination, and effective-
time sorting remain. The bank-sync trigger returns `202` because it only
publishes work. Account, transaction, connection, provider-snapshot, and trigger
response models otherwise remain shared and unchanged.

### Derive requester source and idempotency before finance publication

Add `integration` to the valid finance `CommandRequester` and jobs requester
sources. The HTTP controller derives source from `Caller`: session is
`operator`, access token is `integration`; scheduled code continues creating
`system` requests directly. Source is never accepted in request input.

Bank sync, classification, and transfer-matching submission params gain required
requester source and optional internal idempotency key. They validate HTTP-origin
sources as `operator | integration`, carry both into the existing semantic
command, and do not alter execution handlers.

Each trigger accepts an optional `Idempotency-Key` header containing 1–128
printable ASCII characters. The controller hashes the client key and constructs:

```text
api:access-token:<token-id>:<operation>:<sha256(client-key)>
api:session-user:<user-id>:<operation>:<sha256(client-key)>
```

The operation segment is stable and distinct for sync, classification, and
transfer matching. The resulting key is passed to
`SemanticCommand.IdempotencyKey`. Existing appdispatch semantics return the
original message ID for the same key, topic, and canonical payload, and return
`ErrPublicationConflict` for a changed topic or payload. Missing headers preserve
the existing fresh-publication behavior. No token value or raw client key enters
the command or logs.

### Enforce caller-scoped job reads in storage queries

Jobs `ListParams` gains required requester-user and allowed-source filters. Job
detail gains a store query by job ID, requester user ID, and allowed requester
sources; inaccessible and absent rows both map to `404`.

- Session callers may read their own `operator` and `integration` jobs.
- Access-token callers may read only their own `integration` jobs.
- A requested `source` list filter is intersected with, never broadens, the
  caller's permitted sources.

The shared user-facing list/detail responses retain safe lifecycle timestamps,
attempt count, type, requester, and sanitized terminal error. `workerId` is
removed from detail but remains in storage and operational logs. Dispatch
payloads, raw messages, provider material, and dead-letter messages remain
absent. The initiating direct client alone treats a just-returned job ID's
initial `404` as pending for the bounded grace period.

### Standardize app-route errors around correlation IDs

The shared OpenAPI `Error` schema requires exactly the safe public fields
`code`, `message`, and `correlationId`; the correlation response header remains.
Parsing, credential, policy, and controller error paths use one mapper with these
status/code classes:

- `400 invalid_request`;
- `401 unauthorized`;
- `403 insufficient_permission` or `tenant_access_denied`;
- `404 not_found`;
- `409 idempotency_conflict`;
- `500 internal_error`.

Tenant membership denial maps to `403`. The mapper logs the original wrapped
error with correlation context but never exposes database, provider, transport,
or credential details in the response.

### Add a self-service Admin token screen

The UI adds protected `#/admin/access-tokens`, links it from the Admin overview
and subnavigation, and updates the UI wireframe. The screen follows the existing
non-finance Admin visual stack and provides:

- compact name, permission, and optional-expiry creation;
- newest-first active, expired, and revoked metadata states;
- rotate and revoke only when valid, each behind confirmation;
- one-time create/rotate secret display, explicit copy, and warning;
- loading, empty, error, pending, success, and recoverable mutation states.

The one-time value lives only in page memory. Dismiss, navigation, or reload
removes it; it is not placed in local storage, URLs, analytics, or logs and is
copied only by explicit action. UI tests use faker fixtures; responsive,
keyboard, and visual smoke checks follow module instructions.

### Add `swmd` as an HTTP-only client entrypoint

`apps/sumweave/cmd/swmd` owns Cobra command composition and
`apps/sumweave/internal/swmdclient` owns configuration, authenticated JSON HTTP,
shared error decoding, correlation diagnostics, route construction, output, and
job polling. It reuses generated `v1routes/models` response types but imports no
finance persistence, server wireup, runtime, appdispatch, or jobs store.

The default config is `sumweave/swmd.json` under `os.UserConfigDir()`. The parent
is created with `0700` and the file is atomically written with `0600`. Resolution
precedence is explicit `--base-url`, then `SUMWEAVE_BASE_URL` and
`SUMWEAVE_API_TOKEN`, then `--config` or the default file. The file contains
exactly `baseUrl` and `apiToken`.

Persisted setup uses
`swmd auth configure --base-url <url> --token-stdin`, with optional `--config`
path selection. Both `--base-url` and the presence-only `--token-stdin` flag are
required. `--token-stdin` accepts no value and is the only opt-in that permits
`auth configure` to read a nonempty API token from stdin. Omitting either
required flag, supplying a value to `--token-stdin`, providing no token on stdin,
or passing a secret-bearing `--token` option is invalid: the command exits
nonzero and does not create or modify the selected config file. No `--token`
option is defined. `auth status` redacts the token and calls `/auth/me` unless
`--offline`; `auth clear` removes persisted configuration.

Plain HTTP is accepted only for `localhost`, `127.0.0.1`, and `[::1]`; all other
base URLs require HTTPS. Go's normal TLS verification and finite request timeout
remain enabled. JSON is the only output: one complete value to stdout and
diagnostics to stderr. Authentication, authorization, validation, HTTP,
decoding, failed-job, and timeout outcomes exit nonzero.

The exact command surface is:

```text
swmd auth configure|status|clear
swmd tenant list
swmd account list|get|provider-data-list|provider-data-get
swmd transaction list|get|provider-data-list|provider-data-get
swmd connection list|sync
swmd classification run
swmd transfer match
swmd job list|get|wait
```

Tenant-scoped commands require `--tenant`; identifiers use named flags;
timestamps are validated as RFC 3339 and sent without timezone normalization;
triggers accept `--idempotency-key`. Transaction list advances offset when a
full page is returned. `job wait` has finite interval/timeout settings and treats
`404` as pending only during the first 30 seconds within its overall timeout,
then emits terminal job JSON or fails.

### Keep composition command-specific

`buildHTTP` creates one access-token store/service from the shared app database,
injects validation into credential middleware, and injects lifecycle operations
into the auth controller. `db-migrate` creates the same store and adds it to the
existing migration order after users and refresh tokens. Worker and scheduler
roots do not construct token components. Runtime remains session-only through
the caller adapter. The `swmd` entrypoint resolves only its own client config and
HTTP dependencies; app Viper config does not leak into the client package.

Required dependencies are constructor-enforced; interfaces remain beside their
consumers; errors preserve wrapped causes. Logs use camelCase keys and include
correlation ID, credential kind, token ID where structurally available, user ID,
route, outcome, and accepted job ID where relevant. Bearer values, secrets,
digests, issued responses, provider documents, and credentials are never logged.

### Verify the typical integration scenarios end to end

After automated module and affected checks pass, the implementation completion
gate is one deterministic local smoke run using the repository's PostgreSQL,
split API/worker process model, signed-in browser, and locally built `swmd`:

1. Bootstrap PostgreSQL and sign in as a user with tenant, account, transaction,
   connection, and current provider-snapshot data.
2. Create a read-only token in `#/admin/access-tokens`; capture the one-time value
   and verify later API/UI/storage views expose metadata and digest only.
3. Configure `swmd` with
   `swmd auth configure --base-url <url> --token-stdin`, supplying the token on
   stdin; verify redacted auth status and discover the owning user and current
   tenant memberships.
4. Read accounts, transactions with filters/paging, connections, and account and
   transaction provider-snapshot metadata/details.
5. Verify the read-only token receives `403` from all three write triggers and
   representative session-only finance, runtime, and token-management routes.
6. Create or rotate to a read-write token, submit bank sync, classification, and
   transfer matching, observe the expected initial job `404`, run the bounded
   worker, and use `swmd job wait` through terminal status.
7. Repeat a trigger with the same idempotency key/body and verify one job ID;
   change the body under the same key and verify `409`.
8. Verify another user's, browser-only, system, and unknown jobs return `404` to
   the token while the owning browser session sees only its permitted rows.
9. Rotate the token and verify the old value returns `401` immediately; revoke
   the replacement and verify its next request returns `401` while accepted work
   continues.
10. Inspect API/CLI/UI output and server/job logs to confirm token secrets,
    digests, provider credentials, raw transport payloads, and literal provider
    HTTP bodies are absent.

The Admin screen is also reviewed independently at desktop and narrow widths for
loading, empty, error, active, expired, revoked, create, rotate, one-time result,
confirmation, and keyboard behavior. Any reproducible defect first receives a
focused failing automated test; the affected smoke step and repository
completion checks are then rerun.

## Risks / Trade-offs

- [A leaked long-lived token can act until expiry or revocation] -> Store only a
  high-entropy secret digest, support immediate revocation/rotation, and check
  PostgreSQL on every request.
- [A policy omission can accidentally expose a new route] -> Make session-only
  the default and verify all registered protected operations against the exact
  allowlist with all three caller types.
- [Changing jobs scoping can hide rows currently visible in Admin] -> Apply the
  documented user/source rules to sessions and tokens consistently and cover
  inaccessible detail as indistinguishable `404`.
- [Concurrent rotation can issue several replacements] -> Lock and revoke the
  original row within the replacement transaction and return conflict to later
  attempts.
- [Idempotency keys can collide across callers or operations] -> Scope internal
  keys by stable user/token identity and operation and hash the client key.
- [A newly published job may not yet exist] -> Limit `404`-as-pending behavior to
  a just-returned ID and a 30-second client grace period.
- [A token or provider secret can leak through output or logs] -> Keep issued
  values one-time and in memory, redact config status, preserve provider
  sanitization, and assert safe server/CLI/UI output.
- [Shared error envelopes alter existing client expectations] -> Update the one
  authoritative OpenAPI contract, generated handlers/models, UI clients, and
  registered-route tests together; early alpha does not require compatibility.

## Migration Plan

1. Add access-token persistence and lifecycle logic, then include the exact table
   in the explicit app migration root.
2. Add app caller context and credential-aware middleware while keeping every
   existing operation session-only.
3. Add session-only token-management endpoints and the Admin screen.
4. Mark the exact existing finance reads token-readable and apply bounded
   transaction pagination.
5. Add integration requester source and caller-scoped jobs reads.
6. Add token-write policies and idempotency propagation to the three triggers.
7. Add `swmd` over the shared generated contracts.
8. Bootstrap PostgreSQL, run affected lint/tests, build both Go entrypoints, and
   complete deterministic API/CLI/UI smoke flows from the canonical manual E2E
   sequence.

Deployment runs `sumweave db-migrate` before API startup. Mixed binaries during
the rollout are unsupported in this early-alpha repository. Rollback is a code
rollback; the unused access-token table may remain or be removed by recreating
the development database, and issued tokens stop being accepted by the reverted
API.

## Open Questions

There are no blocking design questions. The canonical integration documents
fully define Phase 1; later roles, OAuth, expanded route access, distribution,
and event-driven integrations remain separate product decisions.
