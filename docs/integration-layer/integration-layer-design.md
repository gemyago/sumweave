# External integration access — system design

Status: draft for discussion. The [product requirements](integration-layer-prd.md)
define expected behavior, and [Architecture](../ARCHITECTURE.md) remains
authoritative.

## Design decisions

Phase 1 uses one application API and one set of finance contracts.

- Personal access tokens are an alternative credential on an explicit allowlist
  of existing routes.
- Existing finance and jobs controllers continue serving both the browser UI and
  direct clients; there is no `/api/v1/integrations` route tree or integration
  controller.
- Signed-in users manage their own tokens through a small screen in the existing
  Admin UI. There is no token-administration CLI or cross-user administrator.
- Token storage uses PostgreSQL types that match the values: `UUID` for IDs,
  `BYTEA` for the SHA-256 digest, and `TEXT` plus constraints for human strings.
- The direct `swmd` client calls the same routes and consumes the same response
  models as the UI.

## Repository findings

The requested operations already have application and finance support:

- app-owned routes are OpenAPI-first in
  `apps/sumweave/internal/api/http/v1routes.yaml` and generated with apigen;
- finance controllers already pass the authenticated user ID as `ActorUserID`;
- finance services enforce current tenant membership before account,
  transaction, connection, provider-snapshot, classification,
  transfer-matching, and synchronization operations;
- the required account, transaction, provider-snapshot, connection, and trigger
  routes already exist under `/api/v1/finance`;
- the current-user route already exists at `/api/v1/auth/me`;
- job list and detail routes already exist under `/api/v1/jobs`, although their
  current store reads are not requester-scoped;
- asynchronous finance operations already publish semantic appdispatch commands
  and return the immutable dispatch message ID;
- observed jobs are materialized on first delivery and already support list and
  detail reads;
- the UI already has a protected Admin area suitable for a small token screen;
- the existing `sumweave` binary is an operational server and administration
  tool, while `swmd` is a separate HTTP-client responsibility.

The implementation therefore needs credential storage, credential-aware route
authorization, self-service token management, requester-scoped job reads,
retry-safe input on the existing triggers, and the direct client. It does not
need duplicate finance handlers or response models.

## Target shape

```text
Signed-in browser                         Direct caller
      |                                      |
Admin > Access tokens                 swmd or HTTPS
      |                               Bearer swat_...
      v                                      |
/api/v1/auth/access-tokens                  |
      |                                      |
      +------------ application API --------+
                           |
                 CredentialAuthMiddleware
                 session JWT or access token
                           |
                 explicit operation policy
                           |
          existing auth / finance / jobs controllers
                    /                    \
            finance services       scoped jobs service
                    |                    |
              finance tables       observed job rows
                    |
          appdispatch semantic commands
                    |
               split worker
```

Browser JWTs retain access to the current application surface. Access tokens
are accepted only for explicitly marked operations in the same surface.

## Credential model

### Token ownership and permission

An access token belongs to exactly one existing auth user. The token does not
copy tenant grants. Once authenticated, its user ID goes through the existing
finance service membership checks.

Permissions remain a small closed enum:

```go
type AccessTokenPermission string

const (
    AccessTokenPermissionReadOnly  AccessTokenPermission = "read-only"
    AccessTokenPermissionReadWrite AccessTokenPermission = "read-write"
)
```

The signed-in user is the operator for their own tokens. Token-management
requests never accept a username or user ID in their payload. A service user
must sign in as itself to create or manage its token. Cross-user administration
requires a future administrator role and is outside this design.

### Application caller

App-owned controllers need more context than the current user-only runtime
identity exposes:

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

Put `Caller` in an app-owned request context. App auth, finance, and jobs
controllers read it directly. Runtime routes receive a narrow adapter exposing
only the user ID, so the generic runtime does not depend on application token
types.

`AccessToken` is required only when `Credential` is `access-token`. The
authentication middleware constructs this invariant; controllers do not repair
missing dependencies or malformed caller state.

### Token format and validation

The issued token format is:

```text
swat_<token-id>_<base64url-secret>
```

- `token-id` is a UUIDv7 in standard hyphenated form.
- The secret is 32 cryptographically random bytes encoded as unpadded base64url.
- Parsing consumes the fixed `swat_` prefix and UUID field before treating the
  remaining text as the secret; `_` is valid base64url data and must not be used
  as an unrestricted split delimiter.
- Store the 32-byte `SHA-256(secret)` digest, never the secret, encoded digest,
  or complete token.
- Select the row by token ID and compare the presented digest with the stored
  digest using a constant-time comparison.
- Reject malformed, unknown, mismatched, expired, revoked, and invalid-permission
  tokens with the same `401` response.
- Return the complete token only from create and rotate responses. Derive the
  display hint from the non-secret ID, for example `swat_018f...`.
- Trim names and require 1–100 characters. Require an optional expiry to be later
  than creation time.

SHA-256 is suitable because the input is a generated 256-bit secret rather than
a human password. Existing Argon2id password hashing remains unchanged.

### Authentication and route authorization

The existing bearer middleware becomes credential-aware:

- a bearer value beginning with the reserved `swat_` prefix is validated only
  as an access token;
- every other bearer value is validated only as a session JWT;
- validators do not fall back to the other credential type after a failure;
- successful validation creates one app-owned `Caller`.

Each protected controller operation wraps its existing handler with one of
three policies:

- session only;
- session or token read;
- session or token write.

This remains one middleware component, constructed once in `buildHTTP` and
injected into the controllers. The policy is an argument at the point where a
controller action is turned into an `http.Handler`; there is not a separately
constructed middleware per action and there is no method/path policy map in the
router. In schematic form:

```go
inner := builder.HandleWith(action)
return c.deps.CredentialAuth.Require(middleware.TokenRead, inner)
```

The generated registration remains unchanged: it calls each controller method
to obtain the final handler and registers that handler for the operation. The
middleware first authenticates the bearer value, puts `Caller` in the request
context, checks that caller against the supplied policy, and invokes `inner`
only when both checks pass. Public operations such as login and refresh remain
unwrapped. Runtime is mounted through the same component with the session-only
policy and receives the narrow runtime identity adapter.

The controllers accept a consumer-defined interface with this single
`Require(policy, next)` operation. The concrete middleware owns credential
parsing and policy evaluation; controller business callbacks only consume the
already-authorized `Caller`. Policy constants are closed and descriptive, so
the controller line is the executable decoration and is easy to review beside
the action. Registered-route tests exercise every protected operation with
session, read-only-token, and read-write-token callers and compare the token-
enabled operations with the complete allowlist in this design.

Session callers satisfy every policy they already satisfy today. Both token
permissions satisfy token read; only `read-write` satisfies token write. A
valid access token on a session-only route receives `403`.

The default is session only. Adding a route and applying ordinary
authentication does not expose it to access tokens. The policy is not inferred
from `GET`, `POST`, tags, or a route prefix.

The OpenAPI document defines one `BearerAuth` HTTP security scheme because both
credentials use the same wire format. The operation description documents token
availability, while registered-route tests verify the executable policy.

## Database schema

Add one app-owned authentication table through the explicit `db-migrate` path.
No finance, appdispatch, jobs, or runtime table is added or changed for token
storage.

The logical table is `auth_access_tokens`; with the default application prefix
it is `sumweave_auth_access_tokens`.

Columns are:

- `id UUID PRIMARY KEY` — UUIDv7 token ID embedded in the token;
- `user_id UUID NOT NULL` — owning auth user ID;
- `name TEXT NOT NULL` — operator-facing name;
- `permission TEXT NOT NULL` — `read-only` or `read-write`;
- `secret_hash BYTEA NOT NULL` — raw 32-byte SHA-256 digest;
- `expires_at TIMESTAMPTZ NULL` — optional expiry instant;
- `revoked_at TIMESTAMPTZ NULL` — immediate logical revocation;
- `created_at TIMESTAMPTZ NOT NULL`;
- `updated_at TIMESTAMPTZ NOT NULL`.

Constraints and indexes are:

- `CHECK (char_length(name) BETWEEN 1 AND 100)`;
- `CHECK (permission IN ('read-only', 'read-write'))`;
- `CHECK (octet_length(secret_hash) = 32)`;
- unique partial index `idx_auth_access_tokens_active_user_name` on
  `(user_id, name) WHERE revoked_at IS NULL`;
- non-unique index `idx_auth_access_tokens_user_created` on
  `(user_id, created_at DESC, id DESC)`.

PostgreSQL does not preallocate the declared maximum for `VARCHAR(n)`: short
strings use their actual length plus a small header. Its documentation states
that `TEXT` and `VARCHAR` have the same general performance, while the
blank-padded `CHAR(n)` type incurs extra storage. `TEXT` is used here to avoid
arbitrary storage-size implications, with explicit constraints where length is
a product invariant. Native `UUID` and `BYTEA` are chosen because they model the
identifier and binary digest directly. See PostgreSQL's
[character type](https://www.postgresql.org/docs/current/datatype-character.html)
and [UUID type](https://www.postgresql.org/docs/current/datatype-uuid.html)
documentation.

Current auth users are always assigned UUIDv7 values, although the existing
`auth_users.id` column is text-compatible. This slice parses that ID before
persisting `user_id`; it does not broaden the work into an auth-schema migration.
There is no foreign key because the existing column types differ and the
application currently has no user-deletion operation. The access-token service
still validates the user through `UserStore` where needed.

There is no plaintext-token column, copied tenant ID, refresh token, per-route
scope, last-used timestamp, or soft-delete column. Revoked metadata remains
available to its owner.

`AccessTokenStore.AutoMigrate()` creates the table and idempotently establishes
the named constraints and indexes after users and refresh tokens migrate. The
store receives the shared `*sql.DB`, application DSN, table prefix, and logger.
The service receives the store, user reader, ID generator, clock, random reader,
and logger through explicit wireup.

## Token lifecycle

`AccessTokenService` owns create, list, rotate, revoke, and validate behavior.
The store owns persistence only.

- Create uses the caller's user ID, inserts one active row, and returns metadata
  plus the one-time token.
- List returns all rows owned by the caller, newest first. It derives
  `active`, `expired`, or `revoked` status and never returns `secret_hash`.
- Revoke updates an owned token's `revoked_at`. Repeating revoke for that same
  owned row succeeds without changing the original timestamp.
- Rotate locks one owned active row, revokes it, and inserts a replacement in one
  database transaction. The replacement keeps the name and permission; the
  caller supplies a new nullable expiry. The response contains the new token
  once.
- A missing or non-owned management ID returns the same `404`.
- An expired or revoked token may be listed but cannot be rotated; the user can
  create a new token instead.

Concurrent rotations serialize on the original row. After one succeeds, a
second attempt sees a revoked token and returns a conflict rather than creating
another replacement.

## Self-service management API and UI

### Existing API additions

Add four browser-session-only operations to the existing `auth` controller:

- `GET /api/v1/auth/access-tokens` lists the signed-in user's token metadata;
- `POST /api/v1/auth/access-tokens` creates a token and returns `201`;
- `POST /api/v1/auth/access-tokens/{tokenId}/rotate` rotates a token and returns
  `200`;
- `DELETE /api/v1/auth/access-tokens/{tokenId}` revokes a token and returns
  `204`.

Create accepts `name`, `permission`, and optional `expiresAt`. Rotate accepts a
required nullable `expiresAt`; name and permission are preserved. Create and
rotate return one shared `AccessTokenIssuedResponse` containing metadata and
`apiToken`. List uses `AccessTokenListResponse` with metadata only.

`AccessTokenMetadata` contains required `id`, `name`, `hint`, `permission`,
`status`, `createdAt`, and `updatedAt`, with nullable `expiresAt` and `revokedAt`.
`AccessTokenIssuedResponse` contains required `token` metadata and the one-time
`apiToken`. `AccessTokenListResponse` contains required `items`.

No management request accepts an owner ID. Access tokens receive `403` from all
four operations.

### Admin screen

Add protected route `#/admin/access-tokens` and link **Access tokens** from the
Admin overview and subnavigation. The Admin area currently represents an
authenticated diagnostics surface rather than a privileged role; the screen is
therefore explicitly self-service.

The screen contains:

- a compact create form for name, permission, and optional expiry;
- a newest-first list showing name, hint, permission, created time, optional
  expiry, and derived status;
- rotate and revoke actions only where valid;
- confirmations explaining that the old credential stops working immediately;
- a one-time result after create or rotate with a copy action and warning that
  the value cannot be recovered.

The one-time value exists only in page memory. Dismissing the result, navigating
away, or reloading removes it. It is not written to local storage, logs, URLs,
analytics, or the clipboard without an explicit copy action.

## Existing HTTP API reuse

Add credential policy to the authoritative
`apps/sumweave/internal/api/http/v1routes.yaml` operations and their existing
controller methods. No route below is duplicated.

### Current user and tenant discovery

Token read is allowed for:

- `GET /api/v1/auth/me`;
- `GET /api/v1/finance/tenants`.

Extend `UserInfo` with optional `accessToken` metadata. It is present for an
access-token caller and omitted for a browser session. The existing tenant list
response remains unchanged.

### Account reads

Token read is allowed for:

- `GET /api/v1/finance/tenants/{tenantId}/accounts`;
- `GET /api/v1/finance/tenants/{tenantId}/accounts/{accountId}`;
- `GET /api/v1/finance/tenants/{tenantId}/accounts/{accountId}/provider-snapshots`;
- `GET /api/v1/finance/tenants/{tenantId}/accounts/{accountId}/provider-snapshots/{snapshotId}`.

Reuse `FinanceAccount`, `FinanceAccountsResponse`,
`FinanceProviderSnapshotMetadata`, `FinanceProviderSnapshotListResponse`, and
`FinanceProviderSnapshot` without aliases.

### Transaction reads

Token read is allowed for:

- `GET /api/v1/finance/tenants/{tenantId}/transactions`;
- `GET /api/v1/finance/tenants/{tenantId}/transactions/{transactionId}`;
- `GET /api/v1/finance/tenants/{tenantId}/transactions/{transactionId}/provider-snapshots`;
- `GET /api/v1/finance/tenants/{tenantId}/transactions/{transactionId}/provider-snapshots/{snapshotId}`.

The existing list query keeps `accountId`, `source`, `status`, `kind`,
`startDate`, `endDate`, `sort`, `includeHidden`, `limit`, and `offset`. Make
`limit` default to 100 and constrain it to 1–200 for every caller. Keep
`FinanceTransactionsResponse`; `swmd` advances the offset when it receives a
full page and does not require an integration-only pagination envelope.

### Connection read and write

Token read is allowed for:

- `GET /api/v1/finance/tenants/{tenantId}/connections`.

Token write is allowed for:

- `POST /api/v1/finance/tenants/{tenantId}/connections/{connectionId}/sync`.

The sync request and response remain the existing shared finance models. Change
the shared route status to `202` because it only publishes asynchronous work;
the browser and access-token clients then observe the same contract.

### Classification and transfer matching

Token write is allowed for:

- `POST /api/v1/finance/tenants/{tenantId}/transactions/classify`;
- `POST /api/v1/finance/tenants/{tenantId}/transactions/match-transfers`.

Reuse `FinanceTransactionClassificationRequest`,
`FinanceClassificationJobResponse`, `FinanceTransferMatchingRequest`, and
`FinanceTransferMatchingJobResponse`. Both operations keep their required
`rangeStart` and `rangeEndExclusive` timestamps and preserve their offsets.

The three write operations return their existing response shapes. `swmd` reads
the common `jobId` field rather than requiring a new integration-only job
response.

### Everything else

All other application, finance, auth, token-management, administration, and
runtime operations remain session only. A registered-route test holds the
complete token allowlist so accidental exposure fails verification.

## Requester-scoped jobs

Token read is allowed on the existing routes:

- `GET /api/v1/jobs`;
- `GET /api/v1/jobs/{jobId}`.

Both browser-session and access-token reads become caller-scoped:

- a session caller is restricted to `requester_user_id = caller.UserID` and the
  `operator` or `integration` requester sources;
- an access-token caller is restricted to `requester_user_id = caller.UserID`
  and the `integration` requester source;
- requested `source` filters are intersected with those allowed sources;
- a missing or inaccessible job returns `404` without distinguishing the cases.

Extend `jobs.ListParams` with required requester-user and allowed-source filters.
Add a store read by job ID, requester user ID, and allowed requester sources so
authorization happens in the query rather than after loading another user's
row. No jobs schema change is required because the columns already exist.

Reuse `JobListResponse`, `JobSummary`, and `JobDetailResponse`. Remove
`workerId` from the shared user-facing detail contract; it remains available in
the store and logs for operations. The remaining requester data can only echo
the current caller and its allowed source. Dispatch payloads, results, raw
messages, provider credentials, and dead-letter data remain absent.

## Shared errors

Use one error contract and mapper for existing application routes. Extend the
shared `Error` schema to require:

```json
{
  "code": "tenant_access_denied",
  "message": "The credential cannot access this tenant.",
  "correlationId": "request-correlation-id"
}
```

The correlation ID also remains in the response header. The shared middleware,
including bearer-auth failures, writes this envelope. There is no token-specific
error mapper.

Required mappings are:

- `invalid_request` for `400`;
- `unauthorized` for `401`;
- `insufficient_permission` or `tenant_access_denied` for `403`;
- `not_found` for `404`;
- `idempotency_conflict` for `409`;
- `internal_error` for `500`.

The mapper exposes no wrapped database, provider, transport, or credential
details. It logs the original wrapped error with the correlation ID. Tenant
membership denial becomes `403` rather than the current unauthorized mapping.

## Durable execution and idempotency

Add `integration` as a valid `CommandRequester` source. Existing finance
controllers derive requester source from `Caller`:

- browser session becomes `operator`;
- access token becomes `integration`;
- scheduled services continue constructing `system` commands directly.

The source is never accepted from an HTTP request.

Add the optional `Idempotency-Key` header to the three existing trigger
operations. It must contain 1–128 printable ASCII characters. For an access
token, the controller constructs:

```text
api:access-token:<token-id>:<operation>:<sha256(client-key)>
```

Browser calls currently omit the header. If a browser caller supplies it, scope
the internal key to the stable user ID rather than the short-lived JWT:

```text
api:session-user:<user-id>:<operation>:<sha256(client-key)>
```

Pass the internal key into the finance submission params and then into
`SemanticCommand.IdempotencyKey`. When the header is absent, preserve the
existing new-publication behavior.

Appdispatch already binds an idempotency key to topic and payload hash. The same
semantic request returns the original message ID; a different topic or payload
returns `ErrPublicationConflict`, mapped to `409`.

Add `RequesterSource` and optional `IdempotencyKey` to the submission params for
bank sync, classification, and transfer matching. Validate requester source as
`operator | integration`. The worker and observed-job registration remain
unchanged apart from accepting the new source string.

## Provider source data boundary

No new provider ingestion or persistence is required. The existing finance
controller calls `ProviderSnapshotService`, which already:

- verifies tenant membership;
- checks account or transaction attachment within that tenant;
- returns metadata without `DocumentJSON` on list;
- sanitizes the JSON document again on detail reads.

The existing API mapping continues to put `DocumentJSON` into
`FinanceProviderSnapshot.data`. It does not expose connector structs, response
headers, connection secrets, or literal HTTP bodies.

## Direct CLI

Add a second Go entrypoint at `apps/sumweave/cmd/swmd`. It is an HTTP client and
must not import finance persistence, server wireup, runtime, appdispatch, or job
stores.

Its client package owns:

- configuration resolution;
- bearer authentication and JSON HTTP requests;
- reuse of the generated `v1routes/models` response types;
- correlation-ID and shared API error handling;
- JSON output and job polling.

### Configuration

The default file is `sumweave/swmd.json` below `os.UserConfigDir()`. Create it
atomically with mode `0600`; create its parent directory with mode `0700` when
needed.

Resolution order, highest precedence first, is:

1. explicit `--base-url` for the non-secret URL;
2. `SUMWEAVE_BASE_URL` and `SUMWEAVE_API_TOKEN`;
3. the file selected by `--config` or the default path.

The file contains exactly `baseUrl` and `apiToken`. There are no profiles or
environment layers. `auth status` redacts the configured token and calls the
existing `/api/v1/auth/me` route unless `--offline` is supplied.

Use `swmd auth configure --base-url ... --token-stdin` for persisted setup so a
token need not appear in shell history or process arguments. For non-persisted
automation, use `SUMWEAVE_API_TOKEN`; Phase 1 has no token command-line flag.

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

Every resource command requires explicit `--tenant` where the route contains a
tenant ID. IDs use named flags. Timestamp flags accept RFC 3339 strings and are
sent without timezone normalization. Trigger commands accept an optional
`--idempotency-key`.

JSON is the only Phase 1 data format. stdout contains one complete JSON value;
progress and diagnostics use stderr.

`job wait` has a finite default timeout and configurable `--interval` and
`--timeout`. Invoking wait is the caller's explicit assertion that the supplied
ID is an expected job reference. It treats `404` as pending only during a
30-second materialization grace period within the overall timeout; a later
`404` is terminal not-found. It exits nonzero for a failed job or timeout.

The CLI can be run with `go run ./cmd/swmd` or built directly from the
`apps/sumweave` module. Release and npm distribution pipelines remain unchanged.

## Package and composition changes

Implementation responsibilities are:

- `internal/auth/access_token_store.go`: access-token persistence only;
- `internal/auth/access_token_service.go`: lifecycle and validation behavior;
- `internal/auth/caller.go`: app-owned caller and request-context values;
- `internal/api/http/middleware/auth.go`: credential selection, validation, and
  explicit route policy;
- `internal/api/http/v1controllers/auth.go`: current-user and token-management
  operations;
- `internal/api/http/v1controllers/finance.go`: existing finance operations with
  selected token policies and derived requester metadata;
- `internal/api/http/v1controllers/jobs.go`: existing requester-scoped job
  operations;
- `internal/jobs`: requester-scoped list and detail storage;
- `cmd/swmd`: direct client commands and output;
- `internal/swmdclient`: configuration and HTTP transport used by
  `swmd`;
- `apps/sumweave-ui/src/pages/AdminAccessTokens.svelte`: self-service screen;
- `apps/sumweave-ui/src/lib/auth`: token-management API calls and models.

Only wireup consumes application config. Constructors enforce required stores,
loggers, ID generators, clocks, and random readers. Consumer-defined interfaces
stay next to the controller, middleware, service, or command that uses them.

`buildHTTP` constructs one `AccessTokenStore` and service from the shared
application database. It injects validation into auth middleware and lifecycle
operations into `AuthController`. `db-migrate` constructs the same store and
runs its migration. Worker and scheduler roots do not construct access-token
components. No dedicated access-token command root is added.

## Logging and security

- Never log a bearer value, secret, digest, create response, or rotate response.
- Continue filtering authorization, cookie, and token headers from access logs.
- Log credential kind, token ID when applicable, user ID, permission, route
  pattern, correlation ID, outcome, and accepted job ID with camelCase keys.
- Authentication failures log a bounded reason and token ID only when the token
  was structurally parseable.
- Do not log provider-snapshot documents.
- Token creation, rotation, and revocation log token ID, owning user ID, and
  outcome, never the returned token.
- Use the existing request-body limit; token and trigger bodies are small.
- No route returns password hashes, refresh tokens, connection secrets,
  dispatch payloads, provider credentials, or stored token digests.
- Check revocation from PostgreSQL on every access-token request in Phase 1;
  there is no credential cache to invalidate.

## Verification

### Backend and registered routes

- Token generation stores only a 32-byte digest and returns the secret once.
- Create rejects duplicate active names, invalid permissions, invalid expiry,
  and random-reader failures.
- Management reads and mutations are owner-scoped and session only.
- Rotation is transactional, creates one replacement, and immediately rejects
  the old token.
- Validation covers malformed, unknown, mismatched, expired, revoked, invalid
  permission, and valid tokens without revealing which check failed.
- Every protected route has a policy and the token allowlist exactly matches
  this design.
- Read-only tokens receive `403` for all three token-write routes.
- Finance reads use the token owner's user ID and preserve tenant membership
  checks.
- Provider metadata and detail mapping preserve the sanitization boundary.
- Job list and detail always enforce their caller and source filters in SQL.
- Idempotent retries return one message ID; changed payloads return `409`.
- Shared API errors contain only documented fields and correlation ID.

Use Mockery for dependency mocks and registered routes for controller tests, in
line with module rules.

### UI and direct client

- The UI covers empty, loading, error, active, expired, and revoked list states.
- Create and rotate show the secret once; dismiss, navigation, and reload remove
  it from UI state.
- Revoke and rotate require confirmation and update the list.
- Responsive and keyboard checks cover the Admin access-token screen.
- CLI tests cover config precedence, file modes, redaction, JSON stdout, shared
  API errors, existing route construction, and job wait behavior with
  `httptest` servers.

### Manual end-to-end flow

1. Bootstrap PostgreSQL and sign in as a user with tenant data.
2. Create a read-only token from `#/admin/access-tokens` and copy its one-time
   value.
3. Configure `swmd` against the local API and verify auth status and tenant
   discovery through the existing routes.
4. Read accounts, transactions, connections, and provider-source data.
5. Verify all session-only and token-write routes reject the read-only token.
6. Create or rotate to a read-write token, trigger sync, classification, and
   transfer matching, and poll the returned jobs.
7. Repeat one trigger with an idempotency key and verify one job ID; change the
   body and verify `409`.
8. Verify jobs from another user or requester source return `404`.
9. Rotate the token and verify the old value returns `401` immediately.
10. Revoke the replacement and verify the next request returns `401`.

## Delivery order

1. Add token storage, lifecycle service, and migration.
2. Extend caller context and bearer middleware with explicit route policies.
3. Add self-service auth endpoints and the Admin access-token screen.
4. Enable the existing finance read routes for token-read policy.
5. Add requester-scoped job reads and the `integration` requester source.
6. Add idempotency and token-write policy to the existing finance triggers.
7. Add `swmd` over the shared API contracts.
8. Complete registered-route, module, affected, UI, and manual end-to-end checks.
