# External integration access — product requirements

Status: draft for discussion. The [system design](integration-layer-design.md)
defines the implementation shape, and [Architecture](../ARCHITECTURE.md) remains
authoritative for product boundaries.

## Goal

Let a person or automation use a long-lived personal access token to read core
Sumweave finance data and start a small set of finance operations without a
browser session.

The first release adds personal access tokens as an alternative credential for
an explicit allowlist of existing HTTP API operations. It also provides a
direct CLI for humans, scripts, and coding agents. It does not create a second
integration API, general-purpose RBAC, or token access to every application
route.

## Users and primary workflows

The initial users are:

- a signed-in member creating and managing their own access tokens in Sumweave;
- a member exporting transactions and accounts into another system;
- an automation periodically starting bank synchronization;
- a finance workflow running classification or transfer matching for a range;
- a script or coding agent starting asynchronous work and waiting for its job.

A dedicated service user may own a token when an integration should not act as
a person. The service user receives tenant access through the existing
membership flow and manages its own tokens through the same signed-in UI.

## Product requirements

### 1. Personal access tokens

- A signed-in user can create a named token for their own user identity.
- Creation selects exactly one permission: `read-only` or `read-write`.
- A token may have an expiration time. A token without one remains valid until
  revoked.
- The complete token value is shown exactly once after creation or rotation.
- The token-management UI lists metadata and status without revealing stored
  token values or hashes.
- A user can rotate or revoke one of their tokens. Rotation immediately revokes
  the old token and returns one replacement token.
- Revocation and rotation take effect on the next request without restarting
  the API.
- A token authenticates as its owning user and can reach only tenants of which
  that user is currently a member.
- Membership changes take effect immediately; tenant IDs are not copied into a
  token.
- Only a browser session can create, list, rotate, or revoke tokens. An access
  token cannot manage itself or other tokens.
- Existing username/password and browser-session behavior remains unchanged.

There is no privileged cross-user token administrator in Phase 1. The existing
application has no administrator role model, so allowing one signed-in user to
mint credentials for another would create an undefined privilege boundary.

### 2. Reuse of the application API

Access tokens use the existing `/api/v1/auth`, `/api/v1/finance`, and
`/api/v1/jobs` routes and their existing resource contracts. There is no
`/api/v1/integrations` route tree and no duplicate integration controller.

Each protected operation declares whether it is:

- browser-session only;
- available to `read-only` and `read-write` tokens; or
- available only to `read-write` tokens.

A new operation is browser-session only until token access is deliberately
declared. Token access is never inferred from the HTTP method or route prefix.

### 3. Permissions and discovery

`read-only` permits only the reads listed in this document.

`read-write` includes those reads and may also:

- start synchronization for an existing bank connection;
- start classification over a transaction timestamp range;
- start transfer matching over a transaction timestamp range.

An authenticated token can use the existing current-user operation to inspect
its owning user and token metadata, including token ID, name, permission, and
optional expiry. It can use the existing finance-tenant list to discover the
tenants available to its owner.

Phase 1 does not grant token access to transaction or account edits, connection
linking or deletion, schedule changes, classification-rule management, manual
transfer links, imports, tenant administration, token administration, reports,
FX administration, or agent-runtime operations.

### 4. Transactions

An access token can use the existing finance operations to:

- list transactions in a tenant with bounded offset pagination;
- filter by account, source, status, kind, effective timestamp range, and hidden
  state;
- choose ascending or descending effective-time order;
- get one transaction by ID;
- list current provider-snapshot metadata for one transaction;
- get one current provider-snapshot document.

Transaction responses retain the shared finance contract, including normalized
ledger fields and the existing provider-original amount, currency, description,
and effective timestamp when available. Provider documents remain separate
resources so ordinary list responses stay bounded.

### 5. Accounts

An access token can use the existing finance operations to:

- list visible accounts in a tenant and optionally include hidden accounts;
- get one account by ID;
- list current provider-snapshot metadata for one account;
- get one current provider-snapshot document.

Account responses retain the shared finance contract, including current booked
and pending balances and existing provider identifiers.

### 6. Provider source data

The product term "provider source data" means Sumweave's current provider
snapshot: credential-stripped, schema-derived JSON captured from a provider
observation. It is not an unmodified HTTP response or a historical response
archive.

- Both token permissions may read source data for reachable tenants.
- Snapshot list operations return metadata only.
- Snapshot detail operations return the JSON document.
- Credentials, authorization material, tokens, signatures, private keys,
  passwords, and comparable secret fields are never returned.
- Snapshot documents continue to be sanitized before persistence and again
  before an API response.
- Account and transaction ownership is checked together with tenant membership;
  an ID from another tenant must not reveal data.

### 7. Connections and synchronization

An access token can use the existing finance route to list a tenant's bank
connections and their safe operational state, including the current schedule
projection and last synchronization diagnostics.

A `read-write` token can start synchronization for one existing connection.
The request may provide a reason and optional start or end timestamp using the
existing bank-sync window semantics. It cannot create, relink, rename, delete,
pause, resume, or reschedule a connection.

Starting synchronization returns the existing asynchronous job reference. It
never runs provider work in the HTTP request.

### 8. Classification and transfer matching

A `read-write` token can start either existing operation for one tenant and one
half-open timestamp range:

```text
rangeStart <= transaction effective timestamp < rangeEndExclusive
```

- Both boundaries are required RFC 3339 timestamps with offsets.
- The start must be earlier than the exclusive end.
- Classification uses the tenant's rules and the existing eligibility and
  non-overwrite behavior.
- Transfer matching uses the existing candidate selection, matching, and
  conservative update behavior.
- Both operations publish durable semantic commands and return their existing
  job references.
- Repeated execution remains safe according to the current classification and
  transfer-matching rules.

### 9. Jobs

An access token can use the existing jobs routes to list and read only jobs
requested with an access token by the same owning user.

- Job status is `queued`, `running`, `succeeded`, or `failed`.
- Responses contain safe lifecycle timestamps, attempt count, job type, and
  sanitized terminal error details when present.
- Worker IDs, dispatch payloads, provider credentials, raw transport messages,
  and dead-letter messages are not exposed to token callers.
- A job ID returned by a trigger may initially return `404` until first delivery
  materializes its observed job. The CLI wait operation treats that state as
  pending for a bounded initial grace period.
- Revoking a token prevents further reads but does not cancel work already
  published.

Browser-session job reads are also scoped to the signed-in user; Phase 1 does
not define a cross-user jobs administrator.

### 10. Retry-safe writes

Each asynchronous trigger accepts an optional `Idempotency-Key` header on its
existing route.

- Repeating the same operation with the same token, key, and request returns the
  original job ID and does not publish duplicate work.
- Reusing that token-scoped key for a different request returns a conflict.
- Omitting the header creates a new operation for every accepted request.

### 11. Token-management UI

Provide a simple protected **Access tokens** screen in the existing Admin area.
It must:

- create a token with name, permission, and optional expiry;
- show the complete token once with a clear copy action and one-time warning;
- list the current user's active, expired, and revoked token metadata;
- rotate an active token after confirmation and show the replacement once;
- revoke an active token after confirmation;
- never render a stored token secret or hash after the one-time response is
  dismissed or the page is left.

The screen is self-service despite its placement in the Admin area: every
signed-in user can manage only their own tokens.

### 12. Direct CLI

Provide a separate `swmd` command, conceptually aligned with the direct `bbmd`
CLI from [atlacp](https://github.com/gemyago/atlacp): focused resource command
groups, local authentication configuration, machine-readable output, and no
need for an MCP transport.

Initial configuration contains only:

```json
{
  "baseUrl": "https://sumweave.example.test",
  "apiToken": "<access-token>"
}
```

The CLI must:

- configure, inspect with redaction, and clear the local base URL and token;
- allow environment variables to supply both values without writing a file;
- support every existing API operation allowed to access tokens;
- write JSON results to stdout and diagnostics to stderr;
- return a nonzero exit status for authentication, authorization, validation,
  HTTP, decoding, terminal job failure, and timeout;
- offer a bounded `job wait` command that understands initial materialization
  delay;
- never print a complete configured token after configuration;
- use normal TLS certificate verification and reject non-loopback plain HTTP.

Distribution and installers are not required. Developers can build or run the
CLI from the repository.

### 13. Errors and operability

- Token-accessible routes use the application's shared stable JSON error
  envelope with a safe code, message, and correlation ID.
- Missing, malformed, expired, or revoked credentials return `401`.
- A valid token with insufficient permission, a browser-session-only route, or
  unavailable tenant access returns `403`.
- Missing resources return `404`, subject to the newly returned job behavior.
- Idempotency conflicts return `409`.
- Validation failures return `400`.
- Logs identify the correlation ID, credential kind, token ID when applicable,
  user ID, route, outcome, and job ID where relevant, but never contain token
  values or provider credentials.

## Acceptance scenarios

- A signed-in user creates a `read-only` token in the UI. The complete secret is
  returned once, later list output shows metadata only, and the stored database
  value cannot be used as the bearer token.
- The token can use the existing tenant, account, transaction, connection, and
  provider-snapshot reads only for its user's current tenant memberships.
- The same token receives `403` from every sync, classification, and
  transfer-matching trigger and from every browser-session-only route.
- A `read-write` token starts each supported operation on its existing finance
  route and receives the existing asynchronous job reference.
- Retrying a trigger with the same idempotency key and body returns the same job
  ID; changing the body returns `409`.
- The token can list and read its owner's integration-requested jobs but cannot
  read browser-requested, scheduled-system, another user's, or unknown jobs.
- `swmd job wait` tolerates an initial materialization delay, reports terminal
  job JSON, exits zero on success, and exits nonzero on failure or timeout.
- Rotation invalidates the old token and returns one replacement secret.
  Revocation takes effect on the next request. Neither operation interrupts
  work that was already accepted.
- Literal provider HTTP bodies and credential-like fields never appear in API
  output, CLI logs, server logs, or jobs.

## Phase 1 non-goals

- OAuth client registration, authorization-code flows, refresh tokens, or
  third-party delegated consent
- general roles, cross-user token administration, per-resource scopes,
  tenant-bound token grants, or custom policy expressions
- a separate integration route namespace or duplicate finance response models
- webhooks, event streaming, job cancellation, job results, job progress, or
  dead-letter access
- literal raw HTTP request or response archival
- historical provider snapshots beyond the current snapshot model
- token access to runtime agents, reports, imports, categories, tags, rules
  management, tenant administration, or connection management
- client SDKs, shell completion, installers, package publication, or MCP
