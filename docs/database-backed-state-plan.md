# Database-Backed State Plan

## Goal

Move the remaining durable application state from local files into database
storage so production app, worker, scheduler, and migration pods require no
persistent filesystem volumes.

This work is separate from the
[Release And Deployment Plan](./release-and-deployment-plan.md). It should be
implemented and reviewed independently before production rollout.

No backward-compatible file migration is required. Signal Foundry is early
alpha; local data may be dropped, migrated through normal schema setup, and
reseeded.

## Scope

Move these remaining durable stores:

- users
- refresh tokens
- raw venue payload bodies
- application-selected agent runtime state

Remove persisted JWT key generation. JWT signing material must come from an
external secret in production.

SQLite remains the local-development database. PostgreSQL is required for
production.

## Authentication Storage

Replace filesystem implementations in:

- `apps/signal-foundry/internal/auth/user_store.go`
- `apps/signal-foundry/internal/auth/refresh_store.go`

Use the existing data-layer SQL connection and DSN. Apply an auth-specific
prefix after the configured data-layer prefix.

### Users

Create a prefixed `auth_users` table with:

- `id` as the primary key
- `username` as a required unique value
- `password_hash` as required text
- `created_at`
- `updated_at`

Use explicit GORM column names. Create users with one database insert and map a
unique constraint failure to `ErrUsernameExists`; do not retain the current
scan-before-create race.

Implement direct indexed reads for user ID and username. Keep deterministic
listing order and update passwords with a conditional update that returns
`ErrUserNotFound` when no row exists.

### Refresh Tokens

Create a prefixed `auth_refresh_tokens` table with:

- SHA-256 token hash as the primary key
- indexed user ID
- indexed expiry timestamp
- creation timestamp

Never persist the opaque token.

Replace separate validation and deletion during rotation with an atomic consume
operation. Concurrent attempts to consume the same token must result in one
success and `ErrInvalidRefreshToken` for all other attempts.

Remove unused store operations rather than preserving filesystem-era API shape.
Regenerate Mockery output after changing the consumer-defined auth interfaces.

## JWT Signing Key

Remove filesystem key persistence from `internal/auth/jwt.go` and its DI wiring.

The effective JWT signing key must be supplied through configuration. Production
will inject `APP_AUTH_JWTSIGNINGKEY` from a Kubernetes Secret. Missing required
signing material must fail startup clearly rather than generating pod-local
keys.

Follow the established environment-specific config pattern. Commit an obvious
non-secret local placeholder such as `local-secret-key` in `local.yaml`; nobody
should treat or reuse it as real signing material. Test config should use its own
obvious placeholder, while production continues to require secret injection.

The same configured key remains available to finance connection-secret
encryption through the existing named DI value.

## Raw Payload Body Storage

Replace `runtime/data/LocalRawPayloadBlobStore` with database-backed storage.

Preserve `RawPayloadBlobStore` and the current opaque `payloadBodyRef` API
contract. This avoids coupling the storage transition to an unrelated backend
and UI contract change.

Add a prefixed raw-payload-body table containing:

- stable body reference as the primary key
- payload ID or equivalent stable ownership identifier
- SHA-256 body hash
- body bytes using the database-native binary type
- creation timestamp

GORM should map `[]byte` to SQLite BLOB and PostgreSQL `bytea`; do not hardcode a
dialect-specific blob type.

Storage must remain immutable. Repeating a payload ID returns the already stored
body reference and hash without replacing bytes. A conflicting body is detected
through the existing hash validation behavior.

Implement bounded body preview reads in the database store so metadata list and
detail operations do not load more body data than required. Metadata list
queries must not select body bytes.

Register the database store as both lineage metadata storage and raw payload
body storage. Delete the local blob implementation, path traversal code, file
write code, and obsolete path configuration.

## Agent Runtime Storage

The Signal Foundry application should select database-backed agent runtime
storage for sessions, provider config, and agent profiles.

Keep generic runtime file-storage APIs only if they remain useful to external
runtime embedders. The application deployment path must not select them.

Use separate agent-runtime and data-layer DSNs and prefixes in configuration,
even when both point to the same PostgreSQL database. This keeps ownership and
table names explicit.

Set local and test configurations to SQLite database DSNs. Production config
must not silently inherit a SQLite path; it should fail until PostgreSQL DSNs
are provided.

## Remaining Filesystem Use

After this transition, production durability must not depend on `dataDir`.

Temporary agent workspace files may use an ephemeral path such as
`/tmp/signal-foundry`. They are not durable state and do not require a
persistent volume.

Platform-agent skills remain read-only runtime assets packaged in the image.

These optional features still require separate decisions before use in a
volume-free production pod:

- application-terminated TLS certificate and key files
- file logging
- Enable Banking private-key files
- enabled workspace shell execution with writable workspace requirements

The initial deployment should terminate TLS upstream, log to stdout, keep
workspace execution disabled, and either leave file-based provider integrations
disabled or change them to consume external secret values.

## Configuration Changes

Update application config and providers to:

- remove the raw payload blob path
- remove persisted JWT key fallback
- require database-backed app storage in production
- keep SQLite DSNs in `local.yaml` and test fixtures only
- keep production DSNs empty until injected
- keep table prefixes explicit and non-colliding

Do not put secrets or production DSNs in tracked YAML.

## Migration Integration

Extend `signal-foundry db-migrate` with an authentication migration step.

The expected ordering is:

1. Agent runtime schema.
2. Data-layer schema, including raw payload bodies.
3. Authentication schema.
4. App dispatch transport.
5. Durable jobs.
6. Finance and remaining product schemas.

Migrations must be idempotent and continue wrapping failures with the component
name. Keep migration tests shallow: one schema smoke path and migration failure
context are sufficient.

No migration from existing JSON files or raw-payload directories is needed.

## Worker Lifecycle

Production worker termination is part of making database-backed durable jobs
operational.

Execute Cobra with a root context canceled by `SIGINT` and `SIGTERM`. The
dedicated `jobs worker` command must pass that context to the consumer so
Kubernetes shutdown can stop polling and active work rather than immediately
terminating the process.

This does not make multiple workers safe. Keep one worker replica until stale
recovery, claims, worker identity, ownership checks, and duplicate delivery are
designed for horizontal execution.

## Tests

Use SQLite-backed tests for ordinary persistence behavior and PostgreSQL smoke
coverage for dialect integration.

Authentication coverage must include:

- unique usernames under concurrent creation
- reads by ID and username
- deterministic user listing
- missing-user password updates
- refresh token creation without storing raw tokens
- expiry rejection
- atomic consume-once behavior
- concurrent consumption of one token

Raw payload coverage must include:

- exact body persistence and retrieval
- stable reference and hash generation
- immutable repeated storage
- bounded previews
- metadata queries that exclude body bytes
- configured table prefixes
- SQLite and PostgreSQL binary data behavior

Migration coverage must confirm auth and raw payload tables are created and a
second migration succeeds.

Regenerate Mockery mocks rather than introducing handwritten test doubles.

## Verification

1. Run affected unit and integration tests.
2. Run `db-migrate` against a fresh local SQLite database.
3. Reseed the first `.local-users` entry and verify login and token rotation.
4. Record and read a raw payload body through the lineage service.
5. Repeat migration and smoke behavior against PostgreSQL.
6. Start API and worker processes with no persistent data directory.
7. Confirm all durable state survives process replacement through database reads.
8. Run `make affected-lint-test`.

## Completion Criteria

This plan is complete when:

- users are database-backed
- refresh tokens are database-backed and atomically consumed
- raw payload bodies are database-backed
- application agent runtime state is database-backed
- JWT signing material is externally configured
- `db-migrate` creates all required schemas
- production pods require no persistent filesystem volume
- local development continues to work with SQLite
- PostgreSQL smoke verification passes
