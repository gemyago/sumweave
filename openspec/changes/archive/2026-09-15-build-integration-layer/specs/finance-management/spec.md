## MODIFIED Requirements

### Requirement: Protected Finance HTTP API

The backend application SHALL expose finance APIs through the existing app under
`/api/v1/finance/...` and SHALL make only the documented read and asynchronous
trigger operations available to personal access tokens.

#### Scenario: Finance APIs are authenticated and tenant-aware

- **WHEN** a caller without a valid authenticated identity calls finance
  endpoints
- **THEN** the system MUST reject the request as unauthorized
- **AND** when a session or allowed token caller accesses finance endpoints,
  tenant-scoped reads and writes MUST remain isolated to the caller's current
  joined tenants.

#### Scenario: Personal access token finance allowlist is exact

- **WHEN** a valid personal access token calls the existing finance API
- **THEN** token read MUST be allowed only for tenant list, account list/detail,
  account current provider-snapshot list/detail, transaction list/detail,
  transaction current provider-snapshot list/detail, and connection list
- **AND** token write MUST be allowed only for bank-connection sync,
  classification submission, and transfer-matching submission
- **AND** every other finance operation MUST remain browser-session only.

#### Scenario: Synthetic pending link configuration is tenant and actor scoped

- **WHEN** an authenticated browser-session tenant member reads or updates
  synthetic link-state configuration for a pending state
- **THEN** the API MUST verify the state belongs to the selected tenant,
  authenticated actor, and provider `synthetic`
- **AND** the API MUST reject expired, consumed, wrong-tenant, wrong-actor, or
  non-synthetic pending states without exposing another user's setup data
- **AND** configuration responses MUST use camelCase JSON and MUST NOT include
  decrypted credentials
- **AND** personal access tokens MUST receive `403` from these operations.

#### Scenario: Finance API covers required product areas

- **WHEN** the first finance slice is implemented
- **THEN** the API surface MUST cover tenants, tenant members/invites, accounts,
  bank connections, synthetic link-state configuration, transactions including
  list/detail/create/update flows, categories, tags, dashboard/reporting, FX
  diagnostics and sync, CSV import preview/confirm/status, and finance-related
  job deep-linking
- **AND** all operator-facing JSON fields MUST use camelCase
- **AND** adding token access MUST NOT duplicate these routes or response models.

## ADDED Requirements

### Requirement: Token-Readable Finance Lists Stay Bounded

Finance list operations available to personal access tokens SHALL reuse the
existing shared list contracts and preserve bounded response behavior.

#### Scenario: Transaction pagination has a shared bound

- **WHEN** any authorized caller lists transactions without a limit
- **THEN** the API MUST use a limit of 100
- **AND** an explicit limit MUST be between 1 and 200
- **AND** sorting MUST occur before offset pagination.

#### Scenario: Existing transaction filters remain available

- **WHEN** a token lists transactions in a reachable tenant
- **THEN** it MUST be able to filter by account, source, status, kind, effective
  timestamp range, and hidden state and choose ascending or descending
  effective-time order
- **AND** the API MUST retain the shared finance transaction response.

#### Scenario: Account and connection reads reuse safe views

- **WHEN** a token lists or reads accounts or lists bank connections
- **THEN** the API MUST reuse the existing finance response models, account
  balance projections, optional hidden-account behavior, safe connection state,
  schedule projection, and last-sync diagnostics
- **AND** it MUST NOT expose connection credentials.

### Requirement: Token Provider Source Reads Preserve Sanitization

Personal access token access to provider source data SHALL use the existing
current provider-snapshot boundary.

#### Scenario: Snapshot metadata and detail stay separate

- **WHEN** a token lists account or transaction provider snapshots
- **THEN** the API MUST return current metadata without the JSON document
- **AND** the existing detail operation MUST return the sanitized schema-derived
  document as a separate resource.

#### Scenario: Snapshot ownership is enforced

- **WHEN** a token supplies an account, transaction, or snapshot ID
- **THEN** the finance service MUST verify current tenant membership and
  finance-object ownership
- **AND** an ID from another tenant MUST NOT reveal data.

#### Scenario: Provider secrets remain absent

- **WHEN** provider source data is persisted or returned
- **THEN** credential-like fields, tokens, signatures, private keys, passwords,
  and authorization material MUST be removed before persistence and checked
  again before the response
- **AND** literal provider HTTP bodies and historical snapshots MUST remain out
  of scope.

### Requirement: Integration Finance Triggers Are Retry-Safe

The three token-writable finance triggers SHALL publish existing semantic
commands with derived requester metadata and optional caller-scoped
idempotency.

#### Scenario: Token starts supported asynchronous work

- **WHEN** a `read-write` token submits bank sync for one existing connection or
  classification or transfer matching for one valid half-open timestamp range
- **THEN** the finance service MUST enforce current tenant membership
- **AND** it MUST publish the existing semantic command with requester user ID
  and source `integration`
- **AND** it MUST return the existing job reference without executing inline or
  creating a job row.

#### Scenario: Read-only token cannot start work

- **WHEN** a `read-only` token calls any of the three trigger operations
- **THEN** the API MUST return `403` without publishing a command.

#### Scenario: Session requester source remains operator

- **WHEN** a browser session submits one of the three triggers
- **THEN** its command requester source MUST be `operator`
- **AND** scheduled services MUST continue constructing `system` requests
  directly
- **AND** requester source MUST never be accepted from HTTP input.

#### Scenario: Idempotent retry returns the original job ID

- **WHEN** the same credential repeats the same operation, request payload, and
  valid `Idempotency-Key`
- **THEN** the API MUST return the original appdispatch message/job ID
- **AND** it MUST NOT publish duplicate work.

#### Scenario: Idempotency key reuse with changed semantics conflicts

- **WHEN** the same credential reuses an `Idempotency-Key` for a different
  operation or request payload
- **THEN** the API MUST return `409 idempotency_conflict`
- **AND** it MUST NOT publish the changed work under that key.

#### Scenario: Idempotency scope is credential-specific

- **WHEN** the controller accepts a 1–128 printable ASCII idempotency key
- **THEN** it MUST scope the internal key to access-token ID or stable session
  user ID plus operation and a SHA-256 hash of the client key
- **AND** it MUST NOT put the complete token or raw client key into commands or
  logs
- **AND** omitting the header MUST preserve one new publication per request.

#### Scenario: Finance trigger range semantics are retained

- **WHEN** a token submits classification or transfer matching
- **THEN** both RFC 3339 timestamp boundaries with offsets MUST be required and
  start MUST be earlier than the exclusive end
- **AND** classification and matching MUST retain their existing eligibility,
  non-overwrite, conservative update, and repeated-execution behavior.

#### Scenario: Bank sync remains asynchronous

- **WHEN** any authorized caller starts an existing connection sync
- **THEN** the shared route MUST return `202` with its existing job reference
- **AND** the optional reason and start/end timestamps MUST retain the existing
  bank-sync window semantics.
