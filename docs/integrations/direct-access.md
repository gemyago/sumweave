---
type: Interface reference
title: "Direct access"
description: "Personal token lifecycle, exact route allowlist, idempotency, and HTTP-only CLI."
sources:
  - id: integration-layer-design
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/docs/integration-layer/integration-layer-design.md"
    title: "docs/integration-layer/integration-layer-design.md"
  - id: access-token-store-go
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/apps/sumweave/internal/auth/access_token_store.go"
    title: "apps/sumweave/internal/auth/access_token_store.go"
  - id: finance-token-allowlist-test-go
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/apps/sumweave/internal/api/http/v1controllers/finance_token_allowlist_test.go"
    title: "apps/sumweave/internal/api/http/v1controllers/finance_token_allowlist_test.go"
  - id: client-go
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/apps/sumweave/internal/swmdclient/client.go"
    title: "apps/sumweave/internal/swmdclient/client.go"
  - id: documentation-snapshot
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/docs/integrations/direct-access.md"
    title: "Reviewed documentation checkpoint before this format migration"
---

# Direct access: personal tokens and swmd

Phase 1 is implemented. Personal access tokens use an explicit allowlist on the
same application routes as the UI. Every finance request uses the token owner's
current membership; a token copies no tenant grants. `read-only` permits allowed
reads and `read-write` also permits exactly three asynchronous triggers. Other
protected operations remain browser-session only. Token management and finance
operational diagnostics support the product even though generic Admin/Chat/model
provider experiments also exist in the executable.[^documentation-snapshot][^integration-layer-design][^access-token-store-go][^finance-token-allowlist-test-go][^client-go]

## Start here

Sign in through the browser and open `#/admin/access-tokens`. Create a token for
your own user, copy the one-time secret, and from `apps/sumweave` configure the
HTTP-only client using explicit stdin:[^documentation-snapshot]

```sh
go run ./cmd/swmd auth configure --base-url http://127.0.0.1:4501 --token-stdin
go run ./cmd/swmd auth status
go run ./cmd/swmd tenant list
go run ./cmd/swmd skill
```

Supply the token through stdin for the configure command. `skill` is an offline,
deterministic command guide; it never resolves credentials or calls the API.
See [local development](../operations/local-development.md) for bootstrap and user
setup and the [integration E2E](../manual-e2e/integration-layer-e2e.md) for a complete
lifecycle and permission-check journey.

## Token format and validation

The issued token format is:[^documentation-snapshot]

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
- Trim names and require 1-100 characters. Require an optional expiry to be later
  than creation time.

SHA-256 is suitable because the input is a generated 256-bit secret rather than
a human password. Existing Argon2id password hashing remains unchanged.

## Authentication policy

The reserved `swat_` prefix selects PAT validation; all other bearer values use
session JWT validation. Neither validator falls back after failure. Invalid
credentials receive a safe `401`; valid tokens on session-only operations or
read-only tokens on token-write operations receive `403`.[^documentation-snapshot]

One app-owned `CredentialMiddleware` authenticates and enforces `SessionOnly`,
`TokenRead`, or `TokenWrite`. Wireup passes policy-bound middleware functions to
controllers as `AuthMiddleware`, `TokenReadMiddleware`, and `TokenWriteMiddleware`.
The default is session only; policies are explicit at each controller operation,
not inferred from HTTP verb or route prefix. The runtime receives only the narrow
user-identity adapter and remains session only.

## Database schema

The app-owned authentication table is prepared through explicit `db-migrate`.
No finance, appdispatch, jobs, or runtime table is added or changed for token
storage.[^documentation-snapshot]

The logical table is `auth_access_tokens`; with the default application prefix
it is `sumweave_auth_access_tokens`.

Columns are:

- `id UUID PRIMARY KEY` - UUIDv7 token ID embedded in the token;
- `user_id UUID NOT NULL` - owning auth user ID;
- `name TEXT NOT NULL` - operator-facing name;
- `permission TEXT NOT NULL` - `read-only` or `read-write`;
- `secret_hash BYTEA NOT NULL` - raw 32-byte SHA-256 digest;
- `expires_at TIMESTAMPTZ NULL` - optional expiry instant;
- `revoked_at TIMESTAMPTZ NULL` - immediate logical revocation;
- `created_at TIMESTAMPTZ NOT NULL`;
- `updated_at TIMESTAMPTZ NOT NULL`.

Constraints and indexes are:

- `CHECK (char_length(name) BETWEEN 1 AND 100)`;
- `CHECK (permission IN ('read-only', 'read-write'))`;
- `CHECK (octet_length(secret_hash) = 32)`;
- unique partial index `idx_auth_access_tokens_active_user_name` on
  `(user_id, name) WHERE revoked_at IS NULL`;
- non-unique index `idx_auth_access_tokens_user_created` on
  `(user_id, created_at DESC)`.

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

`AccessTokenStore.AutoMigrate()` uses the model-defined constraints and indexes
after users and refresh tokens migrate. Listing orders by `created_at DESC, id DESC`;
the index does not include the final ID tie-breaker. The
store receives the shared `*sql.DB`, application DSN, table prefix, and logger.
The service receives the store, user reader, ID generator, clock, random reader,
and logger through explicit wireup.

## Token lifecycle

`AccessTokenService` owns create, list, rotate, revoke, and validate behavior.
The store owns persistence only.[^documentation-snapshot]

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

### Management API

The four browser-session-only operations are to the existing `auth` controller:[^documentation-snapshot]

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

The protected route is `#/admin/access-tokens`; **Access tokens** is linked from the
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

## Exact token allowlist

Credential policies apply to the authoritative
`apps/sumweave/internal/api/http/v1routes.yaml` operations and their existing
controller methods. No route below is duplicated.[^documentation-snapshot]

### Current user and tenant discovery

Token read is allowed for:

- `GET /api/v1/auth/me`;
- `GET /api/v1/finance/tenants`.

`UserInfo` includes optional `accessToken` metadata. It is present for an
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
`startDate`, `endDate`, `sort`, `includeHidden`, `limit`, and `offset`. `limit`
defaults to 100 and is constrained to 1-200 for every caller. The API keeps
`FinanceTransactionsResponse`; `swmd` advances the offset when it receives a
full page and does not require an integration-only pagination envelope.

### Connection read and write

Token read is allowed for:

- `GET /api/v1/finance/tenants/{tenantId}/connections`.

Token write is allowed for:

- `POST /api/v1/finance/tenants/{tenantId}/connections/{connectionId}/sync`.

The sync request and response remain the existing shared finance models. The shared route returns `202` because it publishes asynchronous work;
browser and access-token clients observe the same contract.

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

Token read is allowed on the existing routes:[^documentation-snapshot]

- `GET /api/v1/jobs`;
- `GET /api/v1/jobs/{jobId}`.

Both browser-session and access-token reads are caller-scoped:

- a session caller is restricted to `requester_user_id = caller.UserID` and the
  `operator` or `integration` requester sources;
- an access-token caller is restricted to `requester_user_id = caller.UserID`
  and the `integration` requester source;
- requested `source` filters are intersected with those allowed sources;
- a missing or inaccessible job returns `404` without distinguishing the cases.

Authorization is enforced in requester/source-scoped store queries. The existing
jobs schema already carries these fields; no token-specific job table is added.

Reuse `JobListResponse`, `JobSummary`, and `JobDetailResponse`. The shared user-facing detail contract omits
`workerId`; it remains available in
the store and logs for operations. The remaining requester data can only echo
the current caller and its allowed source. Dispatch payloads, results, raw
messages, provider credentials, and dead-letter data remain absent.

## Shared errors

The accepted application error contract uses a safe JSON envelope:[^documentation-snapshot]

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

The common mappings are:

- `invalid_request` for `400`;
- `unauthorized` for `401`;
- `insufficient_permission` or `tenant_access_denied` for `403`;
- `not_found` for `404`;
- `idempotency_conflict` for `409`;
- `internal_error` for `500`.

The mapper exposes no wrapped database, provider, transport, or credential
details. It logs the original wrapped error with the correlation ID. Tenant
membership denial returns `403`. Generic state conflicts use `409` with `conflict`;
the specialized category-reference conflict includes `ruleIds`.
Classification scenarios retain these shared mappings and the specialized
category-reference conflict contract. Rule create/replace currently have an
implementation gap: explicit `tagIds: null` and non-size-limit body-read failures
return an empty `400` before generated binding. See
[classification](../product/classification.md#verification-and-formal-contracts)
for the accepted contract and current behavior.

## Durable execution and idempotency

`integration` is a valid `CommandRequester` source. Existing finance
controllers derive requester source from `Caller`:[^documentation-snapshot]

- browser session becomes `operator`;
- access token becomes `integration`;
- scheduled services continue constructing `system` commands directly.

The source is never accepted from an HTTP request.

The three trigger operations accept optional `Idempotency-Key`. It must contain
1-128 printable ASCII characters. For an access token, the controller constructs:

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

`RequesterSource` and optional `IdempotencyKey` are submission parameters for
bank sync, classification, and transfer matching. Validate requester source as
`operator | integration`. The worker and observed-job registration remain
unchanged apart from accepting the new source string.

## Provider source data boundary

No new provider ingestion or persistence is required. The existing finance
controller calls `ProviderSnapshotService`, which already:[^documentation-snapshot]

- verifies tenant membership;
- checks account or transaction attachment within that tenant;
- returns metadata without `DocumentJSON` on list;
- sanitizes the JSON document again on detail reads.

The existing API mapping continues to put `DocumentJSON` into
`FinanceProviderSnapshot.data`. It does not expose connector structs, response
headers, connection secrets, or literal HTTP bodies.

## Direct CLI

`apps/sumweave/cmd/swmd` is a separate HTTP client. It does not import finance persistence, server wireup, runtime, appdispatch, or job
stores.[^documentation-snapshot]

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

Plain HTTP is allowed only for localhost or loopback IP literals (including
`127.0.0.1` and `[::1]`). Other hosts require HTTPS. Standard TLS verification
stays enabled, with a default 30-second request timeout. Redirect destinations undergo
the same HTTPS/loopback validation before they are followed. Safe relative
redirects work; credential forwarding also follows Go's standard HTTP redirect
policy. There is no additional same-origin-only policy.

### Command surface

```text
swmd skill
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

`job wait` defaults to a one-second interval and one-minute timeout, configurable
with `--interval` and `--timeout`. The overall deadline also bounds an in-flight poll. Invoking wait is the caller's explicit assertion that the supplied
ID is an expected job reference. It treats `404` as pending only during a
30-second materialization grace period within the overall timeout; a later
`404` is terminal not-found. It exits nonzero for a failed job or timeout.

The CLI can be run with `go run ./cmd/swmd` or built directly from the
`apps/sumweave` module. Release and npm distribution pipelines remain unchanged.


## Security and verification references

Never log bearer values, digests, issued responses, provider-snapshot documents,
or connection credentials. The original wrapped operational error belongs in
logs; user-facing errors carry safe fields and correlation IDs. Token revocation
is checked from PostgreSQL on every request. Revocation blocks the next request
but does not cancel commands already accepted by appdispatch.[^documentation-snapshot]

The [PAT scenarios](../requirements/personal-access-tokens.md),
[direct-client scenarios](../requirements/direct-finance-cli.md),
[finance integration scenarios](../requirements/finance-management.md), and
[job visibility scenarios](../requirements/durable-jobs.md) remain granular
acceptance references. These and the following implementation links are external
to the OKF bundle:

- [Authoritative OpenAPI](../../apps/sumweave/internal/api/http/v1routes.yaml)
- [Token store](../../apps/sumweave/internal/auth/access_token_store.go) and
  [lifecycle service](../../apps/sumweave/internal/auth/access_token_service.go)
- [Credential middleware](../../apps/sumweave/internal/api/http/middleware/auth.go)
- [Registered finance allowlist checks](../../apps/sumweave/internal/api/http/v1controllers/finance_token_allowlist_test.go)
- [Client configuration](../../apps/sumweave/internal/swmdclient/config.go) and
  [HTTP transport](../../apps/sumweave/internal/swmdclient/client.go)

The [historical draft design](../integration-layer/integration-layer-design.md)
preserves original package sketches and delivery/verification plans; its
middleware injection sketch and three-column list index are not current facts.

[^documentation-snapshot]: Reviewed documentation checkpoint, preserved as the source of this section; format and navigation changes do not establish execution results.

[^integration-layer-design]: docs/integration-layer/integration-layer-design.md.
[^access-token-store-go]: apps/sumweave/internal/auth/access_token_store.go.
[^finance-token-allowlist-test-go]: apps/sumweave/internal/api/http/v1controllers/finance_token_allowlist_test.go.
[^client-go]: apps/sumweave/internal/swmdclient/client.go.
