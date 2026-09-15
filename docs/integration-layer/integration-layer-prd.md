# External integration layer — product requirements

Status: draft for discussion. The [system design](integration-layer-design.md)
defines the implementation shape, and [Architecture](../ARCHITECTURE.md) remains
authoritative for product boundaries.

## Goal

Let a person or automation read core Sumweave finance data and start a small set
of finance operations without using a browser login or handling short-lived
session tokens.

The first release provides long-lived personal integration tokens, a deliberately
small HTTP surface, and a direct CLI suitable for humans, scripts, and coding
agents. It does not introduce general-purpose RBAC or expose every existing UI
endpoint.

## Users and primary workflows

The initial users are:

- a member exporting transactions and accounts into another system;
- an automation that periodically starts bank synchronization;
- a finance workflow that runs classification or transfer matching for a range;
- a script or coding agent that starts asynchronous work and waits for its job;
- an operator creating, listing, rotating, or revoking integration credentials.

A dedicated service user may own a token when an integration should not act as a
person. That user receives tenant access through the existing membership flow.

## Product requirements

### 1. Integration access tokens

- An operator can create a named integration token for an existing Sumweave user.
- Creation selects exactly one permission: `read-only` or `read-write`.
- A token may have an expiration time. A token without one remains valid until
  revoked.
- The complete token value is shown exactly once at creation.
- Operators can list token metadata without revealing token values.
- Operators can revoke a token immediately and create a replacement without
  restarting the API.
- A token authenticates as its owning user. It can reach only tenants of which
  that user is currently a member.
- A membership change takes effect for token requests immediately; tenant IDs
  are not copied into the token.
- Tokens are accepted only by the integration API. They do not authenticate the
  browser, agent-runtime, user-administration, or other internal application
  routes.
- Existing username/password and JWT browser sessions continue unchanged.

### 2. Permissions

`read-only` permits only the integration reads listed in this document.

`read-write` includes all reads and may also:

- start a bank-connection synchronization;
- start classification over a transaction timestamp range;
- start transfer matching over a transaction timestamp range.

The permission is an explicit route requirement. Adding a new `GET` or `POST`
route must not grant it to tokens automatically.

Phase 1 does not expose transaction edits, account edits, connection linking or
deletion, schedule changes, classification-rule management, manual transfer
links, imports, tenant administration, or agent-runtime operations.

### 3. Integration discovery

An authenticated integration can:

- inspect the current token identity and permission;
- list the finance tenants available to the token owner.

Tenant discovery is required so a CLI or automation needs only the base URL and
token as initial configuration.

### 4. Transactions

An integration can:

- list transactions in a tenant with bounded pagination;
- filter by account, source, status, kind, effective timestamp range, and hidden
  state;
- choose ascending or descending effective-time order;
- get one transaction by ID;
- list the current provider-source snapshot metadata for one transaction;
- get one current provider-source snapshot document.

Transaction responses include the normalized ledger fields and the existing
provider-original amount, currency, description, and effective timestamp when
available. Provider-source documents remain separate resources so ordinary list
responses stay bounded.

### 5. Accounts

An integration can:

- list visible accounts in a tenant and optionally include hidden accounts;
- get one account by ID;
- list the current provider-source snapshot metadata for one account;
- get one current provider-source snapshot document.

Account responses include current booked and pending balances and existing
provider identifiers.

### 6. Provider source data

The integration term "provider source data" means Sumweave's current provider
snapshot: credential-stripped, schema-derived JSON captured from a provider
observation. It is not an unmodified HTTP response or a historical response
archive.

- `read-only` and `read-write` tokens may read source data for reachable tenants.
- Snapshot list operations return metadata only.
- Snapshot detail operations return the JSON document.
- Credentials, authorization material, tokens, signatures, private keys,
  passwords, and comparable secret fields are never returned.
- Snapshot documents continue to be sanitized before persistence and again
  before an API response.
- Account and transaction ownership is checked together with tenant membership;
  an ID from another tenant must not reveal data.

### 7. Connections and synchronization

An integration can list a tenant's bank connections and their safe operational
state, including the current schedule projection and last synchronization
diagnostics already present in the finance connection response.

A `read-write` token can start synchronization for one existing connection. The
request may provide a reason and optional start or end timestamp using the
existing bank-sync window semantics. It cannot create, relink, rename, delete,
pause, resume, or reschedule a connection.

Starting synchronization returns an asynchronous job reference. It never runs
provider work in the HTTP request.

### 8. Classification and transfer matching

A `read-write` token can start either operation for one tenant and one half-open
timestamp range:

```text
rangeStart <= transaction effective timestamp < rangeEndExclusive
```

- Both boundaries are required RFC 3339 timestamps with offsets.
- The start must be earlier than the exclusive end.
- Classification uses the tenant's rules and the existing eligibility and
  non-overwrite behavior.
- Transfer matching uses the existing candidate selection, matching, and
  conservative update behavior.
- Both operations publish durable semantic commands and return job references.
- Repeated execution remains safe according to the current classification and
  transfer-matching rules.

### 9. Jobs

An integration can list and read only jobs requested through the integration
API by the token owner's user identity.

- Job status is `queued`, `running`, `succeeded`, or `failed`.
- Job responses contain safe lifecycle timestamps, attempt count, job type, and
  sanitized terminal error details when present.
- Worker IDs, dispatch payloads, provider credentials, raw transport messages,
  and dead-letter messages are not exposed.
- A job ID returned by a trigger may initially return `404` until first delivery
  materializes its observed job. Only a client holding that newly returned ID
  treats this as pending.
- Revoking a token prevents further reads, but does not cancel work already
  published.

### 10. Retry-safe writes

Each asynchronous trigger accepts an optional `Idempotency-Key` header.

- Repeating the same operation with the same token, key, and request returns the
  original job ID and does not publish duplicate work.
- Reusing that scoped key for a different request returns a conflict.
- Omitting the header creates a new operation for every accepted request.

### 11. Direct CLI

Provide a separate `swmd` command, conceptually aligned with the direct `bbmd`
CLI from [atlacp](https://github.com/gemyago/atlacp): focused resource command
groups, local authentication configuration, machine-readable output, and no
need for an MCP transport.

Initial configuration contains only:

```json
{
  "baseUrl": "https://sumweave.example.test",
  "apiToken": "<integration-token>"
}
```

The CLI must:

- configure, inspect with redaction, and clear the local base URL and token;
- allow environment variables to supply both values without writing a file;
- support every endpoint in the initial integration API;
- write JSON results to stdout by default and diagnostics to stderr;
- return a nonzero exit status for authentication, authorization, validation,
  HTTP, decoding, and terminal job failures;
- offer a bounded `job wait` command that understands the initial materialization
  delay;
- never print a complete configured token after the configure operation;
- use normal TLS certificate verification and reject non-loopback plain HTTP.

Distribution and installers are not required. Developers can build or run the
CLI from the repository.

### 12. Errors and operability

- Integration errors use a stable JSON envelope with a safe code, message, and
  correlation ID.
- Missing, malformed, expired, or revoked tokens return `401`.
- A valid token with insufficient permission or tenant access returns `403`.
- Missing resources return `404`, subject to the newly returned job behavior.
- Idempotency conflicts return `409`.
- Validation failures return `400`.
- Logs identify the correlation ID, token ID, user ID, route, outcome, and job ID
  where relevant, but never contain token values or provider credentials.

## Acceptance scenarios

- An operator creates a `read-only` token. The complete secret is returned once,
  list output shows metadata only, and the stored database value cannot be used
  as the bearer token.
- The token can list only its user's tenants and can read accounts,
  transactions, and sanitized provider-source documents in those tenants.
- The same token receives `403` from every sync, classification, and
  transfer-matching trigger.
- A `read-write` token starts each supported operation and receives `202` with a
  job ID and job type.
- Retrying a trigger with the same idempotency key and body returns the same job
  ID; changing the body returns `409`.
- The caller can list and read its integration-requested jobs but cannot read an
  operator, scheduled-system, another user's, or unknown job.
- `swmd job wait` tolerates initial `404`, reports the terminal job JSON, exits
  zero on success, and exits nonzero on failure or timeout.
- Revocation takes effect on the next request. Token rotation does not interrupt
  work that was already accepted.
- Literal provider HTTP bodies and credential-like fields never appear in API
  output, CLI logs, server logs, or jobs.

## Phase 1 non-goals

- OAuth client registration, authorization-code flows, refresh tokens, or
  third-party delegated consent
- general roles, per-resource scopes, tenant-bound token grants, or custom policy
  expressions
- a token-management UI or self-service token-management HTTP API
- webhooks, event streaming, job cancellation, job results, job progress, or
  dead-letter access
- literal raw HTTP request or response archival
- historical provider snapshots beyond the current snapshot model
- integration access to runtime agents, reports, imports, categories, tags,
  rules management, tenant administration, or connection management
- client SDKs, shell completion, installers, package publication, or MCP
