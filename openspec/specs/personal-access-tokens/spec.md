# personal-access-tokens Specification

## Purpose
TBD - created by archiving change build-integration-layer. Update Purpose after archive.
## Requirements
### Requirement: Users Manage Their Own Personal Access Tokens

The application SHALL let each signed-in user create and manage only personal
access tokens owned by that same user.

#### Scenario: User creates a token

- **WHEN** a browser-session caller creates a token with a trimmed unique name,
  `read-only` or `read-write` permission, and valid optional future expiry
- **THEN** the application MUST create one token for the caller's user identity
- **AND** it MUST return the complete token exactly once with safe token metadata.

#### Scenario: User lists token metadata

- **WHEN** a browser-session caller lists personal access tokens
- **THEN** the application MUST return only that user's active, expired, and
  revoked token metadata newest first
- **AND** it MUST NOT return complete tokens or stored secret digests.

#### Scenario: User rotates an active token

- **WHEN** a browser-session caller rotates one owned active token with a valid
  nullable replacement expiry
- **THEN** the application MUST revoke the old token and create exactly one
  replacement in one transaction
- **AND** the replacement MUST retain the old name and permission
- **AND** its complete value MUST be returned exactly once.

#### Scenario: Concurrent rotations serialize

- **WHEN** concurrent requests rotate the same active token
- **THEN** at most one request MUST create a replacement
- **AND** later requests MUST observe the revoked original and return conflict.

#### Scenario: User revokes a token

- **WHEN** a browser-session caller revokes one owned token
- **THEN** the token MUST be invalid on the next authenticated request
- **AND** repeating revocation for that same owned token MUST succeed without
  changing its original revocation time.

#### Scenario: Management remains self-service and session-only

- **WHEN** a caller uses an access token for a token-management operation or
  supplies another user's token ID
- **THEN** access-token management MUST return `403`
- **AND** missing and non-owned management IDs MUST both return `404`
- **AND** no management request MUST accept an owner user ID.

### Requirement: Personal Access Token Secrets Are Non-Recoverable

The application SHALL issue high-entropy personal access tokens and persist only
the information needed to validate them without recovering their secrets.

#### Scenario: Token is generated and stored safely

- **WHEN** the application issues a personal access token
- **THEN** it MUST use `swat_<UUIDv7>_<unpadded-base64url-secret>` with 32 random
  secret bytes
- **AND** it MUST store only the raw 32-byte SHA-256 digest of the secret
- **AND** it MUST derive any display hint from the non-secret token ID.

#### Scenario: Presented token is compared safely

- **WHEN** the application validates a structurally valid access token
- **THEN** it MUST select the row by token ID and compare secret digests in
  constant time
- **AND** it MUST check permission, expiry, and revocation from PostgreSQL on
  that request.

#### Scenario: Invalid tokens are indistinguishable

- **WHEN** an access token is malformed, unknown, mismatched, expired, revoked,
  or has an invalid stored permission
- **THEN** the application MUST return the same `401` response without revealing
  which validation failed.

#### Scenario: Credentials do not leak

- **WHEN** token lifecycle, authentication, API, UI, CLI, or logging behavior is
  exercised
- **THEN** complete bearer values, secrets, stored digests, and one-time issued
  responses MUST NOT appear in server logs or later API responses.

### Requirement: Credential Authentication Uses Explicit Operation Policies

The application SHALL authenticate session JWTs and personal access tokens as
distinct credential kinds and authorize protected operations through an
explicit closed policy.

#### Scenario: Credential type is selected without fallback

- **WHEN** a bearer value begins with the reserved `swat_` prefix
- **THEN** the application MUST validate it only as a personal access token
- **AND** every other bearer value MUST be validated only as a session JWT
- **AND** a failed validator MUST NOT fall back to the other credential type.

#### Scenario: Session behavior is retained

- **WHEN** a valid browser-session caller uses an operation it can access today
- **THEN** the operation MUST retain its current session authorization behavior.

#### Scenario: Token read and write permissions are enforced

- **WHEN** a valid token calls an explicitly token-readable operation
- **THEN** both `read-only` and `read-write` MUST be accepted
- **AND** only `read-write` MUST be accepted by an explicitly token-writable
  operation.

#### Scenario: Session-only remains the default

- **WHEN** an access token calls a protected operation not present in the exact
  token allowlist
- **THEN** the application MUST return `403`
- **AND** newly registered protected operations MUST remain session-only until
  deliberately assigned a token policy.

#### Scenario: Runtime stays session-only

- **WHEN** a credential calls the generic runtime route tree
- **THEN** valid session callers MUST continue through the narrow user-identity
  adapter
- **AND** personal access tokens MUST receive `403`
- **AND** the runtime module MUST NOT depend on app-owned token types.

### Requirement: Token Callers Discover Their Identity

An authenticated token caller SHALL be able to discover its owner and safe token
metadata through the existing current-user operation.

#### Scenario: Token calls current user

- **WHEN** a valid access token calls `GET /api/v1/auth/me`
- **THEN** the response MUST identify its owning user
- **AND** it MUST include token ID, name, permission, and optional expiry without
  including the token value or digest.

#### Scenario: Session calls current user

- **WHEN** a browser session calls `GET /api/v1/auth/me`
- **THEN** the existing user response MUST remain available
- **AND** access-token metadata MUST be omitted.

### Requirement: Application API Errors Are Stable And Safe

Token-accessible application routes SHALL use the shared JSON error envelope and
correlation context used by the application API.

#### Scenario: Error envelope is returned

- **WHEN** an app-owned API operation returns an authentication, authorization,
  validation, not-found, conflict, or internal error
- **THEN** the response MUST contain only a safe `code`, `message`, and
  `correlationId`
- **AND** the correlation ID MUST remain available in the response header.

#### Scenario: Status and code are stable

- **WHEN** an app-owned operation rejects a request
- **THEN** `400` MUST use `invalid_request`, `401` MUST use `unauthorized`, `404`
  MUST use `not_found`, `409` idempotency conflicts MUST use
  `idempotency_conflict`, and `500` MUST use `internal_error`
- **AND** `403` MUST use `insufficient_permission` or `tenant_access_denied` as
  applicable.

#### Scenario: Internal details stay in logs

- **WHEN** a wrapped database, provider, transport, or credential error reaches
  the shared mapper
- **THEN** the original error MUST be logged with correlation context
- **AND** its internal details MUST NOT be returned in the public envelope.

### Requirement: Access Token Storage Uses The App Migration Path

The app-owned auth schema SHALL store personal access token metadata and digest
in one explicitly migrated PostgreSQL table.

#### Scenario: Access token table is prepared

- **WHEN** `sumweave db-migrate` prepares the application auth schema
- **THEN** it MUST create the prefixed `auth_access_tokens` table with UUID
  primary key and owner, text name and permission, 32-byte binary secret digest,
  nullable expiry/revocation timestamps, and required creation/update timestamps
- **AND** it MUST apply constraints for name length, permission values, and
  digest length
- **AND** it MUST apply the unique active owner/name and owner/newest indexes.

#### Scenario: Token storage contains no grants or recoverable secret

- **WHEN** a token row is persisted
- **THEN** it MUST NOT contain plaintext token data, copied tenant IDs,
  per-route scopes, refresh tokens, last-used timestamps, or a soft-delete field.

