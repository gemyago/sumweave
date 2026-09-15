# External integration layer — system design

Status: draft for discussion. The [product requirements](integration-layer-prd.md)
define expected behavior, and [Architecture](../ARCHITECTURE.md) remains
authoritative.

## Repository findings

The requested integration operations already have finance-domain support:

- app-owned routes are OpenAPI-first in
  `apps/sumweave/internal/api/http/v1routes.yaml` and generated with apigen;
- finance controllers authenticate browser access with short-lived JWT bearer
  tokens and pass the JWT subject as `ActorUserID`;
- finance services enforce current tenant membership before accounts,
  transactions, connections, provider snapshots, classification, transfer
  matching, and sync operations;
- accounts and transactions already expose normalized records and current
  provider snapshot metadata/detail routes;
- provider snapshots are sanitized and schema-derived, are stored as current
  values, and are not literal raw HTTP responses;
- sync, explicit classification, and explicit transfer matching already publish
  semantic commands through appdispatch and return the dispatch message ID;
- observed jobs are materialized on first delivery and already have cursor-based
  list and detail storage, but the current jobs HTTP reads do not restrict rows
  to the authenticated requester's user ID;
- the existing `sumweave` binary is an operational server/admin CLI, while the
  requested direct API client is a separate responsibility.

The first integration slice can therefore reuse finance services and durable
execution. It needs a machine-credential store, a narrow route namespace,
requester-scoped jobs reads, retry-safe trigger input, and a direct client CLI.

The `bbmd` reference provides the useful CLI conventions: Cobra resource groups,
local auth status/configuration, redacted credentials, JSON stdout, and focused
commands intended for both humans and agents. Phase 1 uses those conventions
with one Sumweave endpoint instead of multiple account profiles.

## Target shape

```text
Operator shell                         Integration caller
      |                                         |
sumweave access-token                 swmd or direct HTTPS
 create/list/revoke                            |
      |                             Authorization: Bearer swat_...
      v                                         |
AccessTokenStore                   /api/v1/integrations/*
      |                                         |
PostgreSQL                         IntegrationAuthMiddleware
                                                |
                                   explicit read/write route policy
                                                |
                                    IntegrationController
                                      /                 \
                              finance services       jobs service
                                      |                 |
                         tenant membership guard   requester filter
                                      |                 |
                               finance tables     observed job rows
                                      |
                       appdispatch semantic commands
                                      |
                                split worker
```

The browser and runtime routes keep JWT authentication. Integration tokens are
accepted only under `/api/v1/integrations`.

## Authentication and caller identity

### Token ownership and permission

An access token belongs to exactly one existing auth user. The token does not
copy tenant grants. Once authenticated, its user ID goes through the existing
finance service membership checks.

Go values are closed enums:

```go
type AccessTokenPermission string

const (
    AccessTokenPermissionReadOnly  AccessTokenPermission = "read-only"
    AccessTokenPermissionReadWrite AccessTokenPermission = "read-write"
)
```

The authenticated integration caller contains:

```go
type IntegrationCaller struct {
    UserID     string
    TokenID    string
    TokenName  string
    Permission AccessTokenPermission
    ExpiresAt  *time.Time
}
```

Put this caller in an app-owned request context. Finance and integration
controllers must not depend on access-token storage types; they read the caller
and pass only `UserID` into finance service params. Existing runtime
`CallerIdentity` remains an adapter for JWT-protected runtime routes rather than
the integration authorization model.

### Token format and validation

The issued token format is:

```text
swat_<token-id>_<base64url-secret>
```

- `token-id` is the existing UUIDv7 string convention.
- The secret is 32 cryptographically random bytes encoded without padding.
- Store `SHA-256(secret)` as lowercase hexadecimal; never store the secret or
  complete token.
- Parsing selects the row by token ID, then compares the presented secret hash
  with the stored hash using a constant-time comparison.
- Reject a malformed token, missing row, revoked row, expired row, unknown
  permission, or hash mismatch with the same `401` response.
- The full token is returned only from the create command. List output renders
  a hint from the non-secret token ID, for example `swat_018f...`.
- Trim names and require 1–100 characters. Require an optional expiry to be
  later than the creation time. A revoked token cannot be reactivated.

SHA-256 is appropriate here because the input is a generated 256-bit secret,
not a human password. Existing Argon2id password hashing remains unchanged.

### Route authorization

Registration marks each integration operation as requiring `read` or `write`.
The integration middleware first authenticates the token, then checks the
operation's declared requirement:

- both permissions satisfy `read`;
- only `read-write` satisfies `write`.

Do not infer access from the HTTP method. The generated controller method wraps
its handler with the explicit requirement so a future route is inaccessible
until its policy is deliberately selected.

JWTs are not accepted by this namespace, and integration tokens are not added as
an alternative to the existing JWT middleware.

## Database schema

Add one app-owned authentication table through the explicit `db-migrate` path.
No finance, appdispatch, or runtime table changes are required.

The logical table name is `auth_access_tokens`; with the default application
prefix it is `sumweave_auth_access_tokens`.

Columns are:

- `id VARCHAR(255) PRIMARY KEY` — UUIDv7 token ID embedded in the token.
- `user_id VARCHAR(255) NOT NULL` — existing auth user ID.
- `name VARCHAR(100) NOT NULL` — operator-facing name.
- `permission VARCHAR(16) NOT NULL` — `read-only` or `read-write`.
- `secret_hash VARCHAR(64) NOT NULL` — lowercase SHA-256 hex digest.
- `expires_at TIMESTAMPTZ NULL` — optional expiry instant.
- `revoked_at TIMESTAMPTZ NULL` — immediate logical revocation.
- `created_at TIMESTAMPTZ NOT NULL`.
- `updated_at TIMESTAMPTZ NOT NULL`.

Constraints and indexes are:

- unique partial index `idx_auth_access_tokens_active_user_name` on
  `(user_id, name) WHERE revoked_at IS NULL`, allowing a revoked name to be
  reused during rotation;
- non-unique index `idx_auth_access_tokens_user_created` on
  `(user_id, created_at DESC, id DESC)`;
- check constraint `chk_auth_access_tokens_permission` restricting permission to
  `read-only` and `read-write`.

There is no plaintext-token column, copied tenant ID, refresh token, per-route
scope, last-used timestamp, or soft-delete column. Revoked metadata remains
available to operators. User creation is validated by `UserStore` before token
creation; the current application has no user-deletion operation, so this slice
does not add a database foreign key.

Add `AccessTokenStore.AutoMigrate()` to the authentication step after users and
refresh tokens. The store receives the shared `*sql.DB`, application DSN,
application table prefix, ID generator, clock, and logger through explicit
wireup.

## Token administration

Extend the operational `sumweave` binary with a narrow `access-token` group:

```text
sumweave access-token create \
  --username integration-user \
  --name accounting-export \
  --permission read-only \
  [--expires-at 2027-01-01T00:00:00+00:00]

sumweave access-token list [--username integration-user]
sumweave access-token revoke --id <token-id>
```

`create` prints a JSON object containing token metadata and the one-time
`apiToken`. `list` prints metadata and a token hint but no hash or secret.
`revoke` is idempotent and emits no token value.

Use a dedicated access-token command root that constructs only the application
database, logger, user store, access-token store/service, ID generator, and
clock. It constructs no HTTP server, finance module, runtime, publisher, worker,
or scheduler.

## HTTP API

Add the integration paths and schemas to the existing authoritative
`apps/sumweave/internal/api/http/v1routes.yaml` under a new `integration` tag.
The dedicated path prefix is `/api/v1/integrations`.

Add this OpenAPI security scheme and apply it to every integration operation:

```yaml
IntegrationAccessToken:
  type: http
  scheme: bearer
  bearerFormat: SumweaveAccessToken
```

The controller reuses existing finance response mappers where the contract is
identical. It depends only on the tenant, catalog, ledger, bank-sync, provider
snapshot, classification, and transfer-matching services plus the scoped jobs
reader.

### Discovery reads

- `GET /api/v1/integrations/me` requires read access and returns token ID, name,
  user ID, permission, and optional expiry.
- `GET /api/v1/integrations/tenants` requires read access and returns the
  existing `FinanceTenantListResponse` for the token owner.

### Account reads

- `GET /api/v1/integrations/tenants/{tenantId}/accounts` requires read access.
  Optional `includeHidden` defaults to false.
- `GET /api/v1/integrations/tenants/{tenantId}/accounts/{accountId}` requires
  read access.
- `GET /api/v1/integrations/tenants/{tenantId}/accounts/{accountId}/provider-snapshots`
  requires read access and returns metadata only.
- `GET /api/v1/integrations/tenants/{tenantId}/accounts/{accountId}/provider-snapshots/{snapshotId}`
  requires read access and returns the sanitized source document.

Reuse `FinanceAccount`, `FinanceAccountsResponse`,
`FinanceProviderSnapshotMetadata`, `FinanceProviderSnapshotListResponse`, and
`FinanceProviderSnapshot` without adding aliases.

### Transaction reads

- `GET /api/v1/integrations/tenants/{tenantId}/transactions` requires read
  access.
- `GET /api/v1/integrations/tenants/{tenantId}/transactions/{transactionId}`
  requires read access.
- `GET /api/v1/integrations/tenants/{tenantId}/transactions/{transactionId}/provider-snapshots`
  requires read access and returns metadata only.
- `GET /api/v1/integrations/tenants/{tenantId}/transactions/{transactionId}/provider-snapshots/{snapshotId}`
  requires read access and returns the sanitized source document.

The list query supports the existing `accountId`, `source`, `status`, `kind`,
`startDate`, `endDate`, `sort`, and `includeHidden` filters. `limit` defaults to
100 and is restricted to 1–200; `offset` defaults to zero. Ordering remains
`effectiveAt`, `createdAt`, then `id`, all in the selected direction.

Return a new `IntegrationTransactionsResponse`:

```json
{
  "items": [],
  "nextOffset": 100
}
```

`nextOffset` is omitted when fewer than `limit` items are returned. This is a
bounded initial export contract built on the current ledger service pagination.
Reuse `FinanceTransaction` for each item.

### Connection reads and sync trigger

- `GET /api/v1/integrations/tenants/{tenantId}/connections` requires read
  access and returns `FinanceConnectionsResponse`.
- `POST /api/v1/integrations/tenants/{tenantId}/connections/{connectionId}/sync`
  requires write access and returns `202`.

The sync body reuses `FinanceConnectionSyncRequest`: optional `reason`,
`windowStart`, and `windowEnd`. Timestamps are passed through with their offsets
and use the existing target-window behavior.

### Classification and transfer-matching triggers

- `POST /api/v1/integrations/tenants/{tenantId}/transactions/classify` requires
  write access and returns `202`.
- `POST /api/v1/integrations/tenants/{tenantId}/transactions/match-transfers`
  requires write access and returns `202`.

Reuse `FinanceTransactionClassificationRequest` and
`FinanceTransferMatchingRequest`. Both require `rangeStart` and
`rangeEndExclusive`, and the controller preserves the full timestamps and
offsets.

All three trigger routes return one new common response:

```json
{
  "jobId": "dispatch-message-id",
  "jobType": "finance.classification"
}
```

The other job types are the existing finance bank-sync and transfer-matching
job type strings. The exact Phase 1 values are
`finance.bank_connection_sync`, `finance.classification`, and
`finance.transfer-matching`.

### Job reads

- `GET /api/v1/integrations/jobs` requires read access and accepts repeatable
  `status` and `jobType`, plus `limit` from 1–100 and the existing opaque
  `cursor`.
- `GET /api/v1/integrations/jobs/{jobId}` requires read access.

Both queries require `requester_user_id = caller.UserID` and
`requester_source = integration`. A missing or non-owned job is `404`; never
distinguish those cases.

Return `IntegrationJob` rather than the operator-facing job detail. Its exact
fields are:

- required `id`, `jobType`, `status`, `createdAt`, `updatedAt`, and
  `attemptCount`;
- optional nullable `startedAt` and `completedAt`;
- optional `error` using the existing safe `JobError` fields `code`, `summary`,
  and `details`.

`IntegrationJobListResponse` contains required `items` and optional
`nextCursor`. It does not contain requester identity, worker ID, schedule
metadata, dispatch payload, result, progress, or dead-letter data.

Extend `jobs.ListParams` with `RequesterUserID`, and add a required-source
filter for the integration reader. Add a store method that gets by job ID,
requester user ID, and requester source in one query. The existing jobs table
already has `requester_user_id` and `requester_source`; no jobs schema change is
required.

### Errors

Document and return this safe envelope for integration routes:

```json
{
  "code": "tenant_access_denied",
  "message": "The token cannot access this tenant.",
  "correlationId": "request-correlation-id"
}
```

New integration error codes are:

- `invalid_request` for `400`;
- `unauthorized` for `401`;
- `insufficient_permission` or `tenant_access_denied` for `403`;
- `not_found` for `404`;
- `idempotency_conflict` for `409`;
- `internal_error` for `500`.

The integration error mapper exposes no wrapped database, provider, transport,
or credential details. It logs the original error with the correlation ID.
Existing app routes retain their current error behavior.

## Durable execution and idempotency

Add `integration` as a valid `CommandRequester` source. The integration
controller supplies it explicitly; existing browser flows keep `operator`, and
scheduled flows keep `system`. Observed job metadata therefore needs no new
field or table column.

For the three integration write routes, accept an optional `Idempotency-Key`
header containing 1–128 printable ASCII characters. The controller constructs
the internal appdispatch key as:

```text
integration:<token-id>:<operation>:<sha256(client-key)>
```

Pass this key into the finance submission params and then into
`SemanticCommand.IdempotencyKey`. When the header is absent, preserve the
existing new-publication behavior.

Appdispatch already binds an idempotency key to topic and payload hash. A repeat
with the same semantic request returns the original message ID; a different
topic or payload returns `ErrPublicationConflict`, mapped to `409`.

Add `RequesterSource` and optional `IdempotencyKey` to the submission params for
bank sync, classification, and transfer matching. Validate requester source as
`operator | integration`; scheduled services continue constructing `system`
commands directly. The controller, not the client, chooses the source value.

The worker and job-observed registration remain unchanged apart from accepting
the new source string. Work is still delivered at least once, and job records
remain lazy projections whose IDs equal dispatch message IDs.

## Provider source data boundary

No new provider ingestion or persistence is required. Integration detail routes
call `ProviderSnapshotService`, which already:

- verifies tenant membership;
- checks the account or transaction attachment within that tenant;
- returns metadata without `DocumentJSON` on list;
- sanitizes the JSON document again on detail reads.

The API maps `DocumentJSON` into `FinanceProviderSnapshot.data`. It does not add
connector client structs, response headers, connection secrets, or HTTP bodies
to the public contract.

## Direct CLI design

Add a second Go entrypoint at `apps/sumweave/cmd/swmd`. It is an HTTP client and
must not import finance persistence, server wireup, runtime, appdispatch, or
jobs stores.

Its small client package owns:

- configuration resolution;
- bearer authentication and JSON HTTP requests;
- typed request/response models matching integration OpenAPI;
- correlation-ID and structured API error handling;
- JSON output and job polling.

### Configuration

The default file is `sumweave/swmd.json` below `os.UserConfigDir()`. The file is
created atomically with mode `0600`; its parent directory is `0700` when the CLI
creates it.

Resolution order, highest precedence first, is:

1. an explicitly supplied `--base-url` for the non-secret URL;
2. `SUMWEAVE_BASE_URL` and `SUMWEAVE_API_TOKEN`;
3. the config file selected by `--config` or the default path.

The file schema contains exactly `baseUrl` and `apiToken`. There are no named
profiles or environment layers. `auth status` redacts the token and calls the
integration `me` endpoint unless `--offline` is supplied.

Use `swmd auth configure --base-url ... --token-stdin` for persisted setup so a
token need not appear in shell history or process arguments. For non-persisted
automation, supply the token through `SUMWEAVE_API_TOKEN`; Phase 1 has no token
command-line flag.

Allow `http://localhost`, `http://127.0.0.1`, and `http://[::1]` for local
development. All other base URLs must use HTTPS. Use Go's default certificate
verification and a finite request timeout.

### Command surface

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

Every resource command requires explicit `--tenant` where the API path contains
a tenant. IDs use named flags to keep scripts readable. Timestamp flags accept
RFC 3339 strings and are sent without timezone normalization.

JSON is the default and only Phase 1 data format. stdout contains one complete
JSON value; progress and diagnostics use stderr. `job wait` polls with a default
two-second interval and finite default timeout, accepts `--interval` and
`--timeout`, treats `404` as pending only for its supplied freshly returned job
ID, and exits nonzero for failed jobs or timeout.

The CLI can be run with `go run ./cmd/swmd` or built directly from the
`apps/sumweave` module. The release and npm distribution pipelines remain
unchanged.

## Package and composition changes

The implementation should use these responsibilities:

- `internal/auth/access_token_store.go`: access-token persistence only.
- `internal/auth/access_token_service.go`: create, list, revoke, and validate
  token behavior.
- `internal/auth/caller.go`: app-owned caller context values.
- `internal/api/http/middleware/integration_auth.go`: bearer parsing and explicit
  read/write policy.
- `internal/api/http/v1controllers/integration.go`: integration route mapping
  and finance-service calls.
- `internal/jobs`: requester-scoped list/get support using the existing store.
- `internal/wireup/access_tokens.go`: narrow operational token command root.
- `cmd/sumweave/access_token_cmd.go`: operator commands.
- `cmd/swmd`: direct API CLI commands and output.
- `internal/integrationclient`: configuration and HTTP client used only by
  `swmd`.

Only wireup consumes application config. Constructors enforce required stores,
loggers, ID generators, clocks, and random readers. Consumer-defined interfaces
remain next to the controller, middleware, service, or command that uses them.

`buildHTTP` constructs one `AccessTokenStore` and service from the shared
application database, then injects the validator into integration middleware.
`db-migrate` constructs the same store and runs its migration. Worker and
scheduler roots do not construct access-token components.

## Logging and security

- Never log the bearer value, secret, or hash.
- Continue filtering `Authorization`, cookie, and token headers from access
  logs.
- Log token ID, user ID, permission, route pattern, correlation ID, outcome, and
  accepted job ID with camelCase keys.
- Authentication failures log a bounded reason and token ID only when the token
  was structurally parseable.
- Do not log provider snapshot documents.
- Token creation and revocation log token ID, owning user ID, and operator
  command outcome.
- Use the existing request-body limit; integration write bodies are small.
- No integration endpoint returns auth-user password hashes, refresh tokens,
  connection secrets, dispatch payloads, or provider credentials.
- Revocation is checked from PostgreSQL on every request in Phase 1. There is no
  credential cache to invalidate.

## Verification

### Unit and registered-route coverage

- Token generation stores only a hash and returns the secret once.
- Create rejects unknown users, duplicate names, invalid permissions, invalid
  expiry, and random-reader failures.
- Validation covers malformed, unknown, mismatched, expired, revoked, and valid
  tokens without revealing which check failed.
- Revocation is immediate and idempotent.
- Every integration route is registered and protected by its declared access
  level.
- Read-only tokens receive `403` for all three write routes.
- Finance reads use the token owner's user ID and preserve tenant membership
  checks.
- Provider metadata and detail mapping preserve the sanitization boundary.
- Job list/get always filter on caller user ID and `integration` source.
- Idempotent retries return one message ID; changed payloads return `409`.
- Integration errors contain only documented fields and include the correlation
  ID.
- CLI config precedence, file modes, redaction, JSON stdout, API errors, and job
  wait behavior use `httptest` servers.

Use Mockery for dependency mocks and registered routes for controller tests, in
line with module rules.

### Manual end-to-end flow

1. Bootstrap PostgreSQL and create or select an existing user with tenant data.
2. Create one `read-only` and one `read-write` token through `sumweave`.
3. Configure `swmd` against the local API and verify `auth status` and tenant
   discovery.
4. Read accounts, transactions, and provider-source metadata/detail.
5. Verify every write command is forbidden with the read-only token.
6. Trigger sync, classification, and transfer matching with the read-write
   token and poll each returned job.
7. Repeat one trigger with an idempotency key and verify one job ID; change the
   body and verify `409`.
8. Verify jobs belonging to another user or source return `404`.
9. Revoke the active token and verify the next request returns `401`.

## Delivery order

1. Add token storage, service, migration, and operational commands.
2. Add integration caller context, authentication, permission checks, and safe
   errors.
3. Add discovery and read endpoints, including provider snapshots.
4. Add requester-scoped job reads and the `integration` requester source.
5. Add idempotent sync, classification, and transfer-matching triggers.
6. Add `swmd` configuration, resource commands, and job waiting.
7. Complete registered-route, module, affected, and manual end-to-end checks.
