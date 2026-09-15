# direct-finance-cli Specification

## Purpose
TBD - created by archiving change build-integration-layer. Update Purpose after archive.
## Requirements
### Requirement: Direct Client Uses The Existing Application API

The repository SHALL provide a separate Go `swmd` HTTP client that supports all
and only operations available to personal access tokens through the existing
Sumweave routes and response contracts.

#### Scenario: Client exposes the Phase 1 command surface

- **WHEN** a user invokes `swmd`
- **THEN** it MUST provide auth configure/status/clear, tenant list, account
  list/get/provider-data-list/provider-data-get, transaction
  list/get/provider-data-list/provider-data-get, connection list/sync,
  classification run, transfer match, and job list/get/wait commands
- **AND** tenant-scoped commands MUST require an explicit tenant ID
- **AND** resource identifiers MUST use named flags.

#### Scenario: Client stays an HTTP-only responsibility

- **WHEN** the direct client is built
- **THEN** it MUST consume the existing API and generated response models
- **AND** it MUST NOT import finance persistence, server wireup, runtime,
  appdispatch, or job stores.

#### Scenario: Client emits machine-readable results

- **WHEN** a command succeeds
- **THEN** stdout MUST contain one complete JSON value
- **AND** diagnostics MUST use stderr
- **AND** authentication, authorization, validation, HTTP, decoding, failed-job,
  and timeout errors MUST return a nonzero exit status.

### Requirement: Direct Client Configuration Protects Credentials

The direct client SHALL resolve a base URL and API token from explicit,
environment, or local-file configuration without displaying or insecurely
persisting the complete token.

#### Scenario: Configuration precedence is deterministic

- **WHEN** configuration values exist in several supported sources
- **THEN** explicit `--base-url` MUST take precedence for the URL, followed by
  `SUMWEAVE_BASE_URL` and `SUMWEAVE_API_TOKEN`, followed by the selected or
  default config file
- **AND** the config file MUST contain exactly `baseUrl` and `apiToken`.

#### Scenario: Explicit stdin opt-in persists private configuration

- **WHEN** a user invokes
  `swmd auth configure --base-url <url> --token-stdin` with a nonempty API token
  on stdin
- **THEN** `--base-url` and the presence-only `--token-stdin` flag MUST both be
  required for persisted setup
- **AND** `--token-stdin` MUST accept no value and MUST cause the command to read
  the token from stdin rather than a token argument
- **AND** it MUST create the config parent with mode `0700` when needed
- **AND** it MUST atomically create the selected config file with mode `0600`.

#### Scenario: Invalid configure invocation cannot persist credentials

- **WHEN** a user omits `--base-url` or `--token-stdin`, supplies a value to
  `--token-stdin`, provides no token on stdin, or passes `--token`
- **THEN** `swmd auth configure` MUST exit nonzero
- **AND** it MUST NOT create or modify the selected config file
- **AND** the command MUST NOT define a secret-bearing `--token` option.

#### Scenario: Status and clear do not disclose the token

- **WHEN** a user inspects or clears local authentication configuration
- **THEN** status MUST redact the complete token and call the existing current-
  user endpoint unless `--offline` is supplied
- **AND** clear MUST remove persisted configuration without printing the token.

#### Scenario: Environment configuration is not persisted

- **WHEN** the base URL and token are supplied through environment variables
- **THEN** resource commands MUST use them without writing a configuration file.

### Requirement: Direct Client Enforces Transport Safety

The direct client SHALL use bounded requests and normal certificate verification
for every remote API call.

#### Scenario: Redirect destinations are validated before credentials are sent

- **WHEN** a credential-bearing direct-client request receives a redirect
- **THEN** the client MUST validate the proposed destination with the same
  loopback-only plain-HTTP and HTTPS requirements as the configured base URL
- **AND** it MUST reject an unsafe destination, including an HTTPS-to-HTTP
  downgrade, before forwarding the `Authorization` header
- **AND** a redirect policy MUST NOT disable normal Go TLS certificate
  verification or the finite request timeout.

### Requirement: Direct Client Preserves Finance Request Semantics

The direct client SHALL construct requests using the shared finance contracts
without changing timestamp or pagination meaning.

#### Scenario: Timestamp flags preserve offsets

- **WHEN** a caller supplies a classification, matching, sync-window, or
  transaction-range timestamp
- **THEN** the client MUST require an RFC 3339 value where the API requires one
- **AND** it MUST send the supplied offset without timezone normalization.

#### Scenario: Trigger retries carry client idempotency

- **WHEN** a trigger command receives `--idempotency-key`
- **THEN** the client MUST send that value in the existing operation's
  `Idempotency-Key` header.

#### Scenario: Transaction list advances bounded pages

- **WHEN** a transaction list response contains a full requested page
- **THEN** the client MUST advance the existing offset to request the next page
- **AND** it MUST NOT require an integration-only pagination envelope.

### Requirement: Direct Client Waits For Observed Jobs Safely

The direct client SHALL provide bounded polling for a known job reference while
respecting lazy observed-job materialization.

#### Scenario: Overall wait deadline bounds an in-flight poll

- **WHEN** `swmd job wait` has an overall timeout and a `GET` poll is still in
  flight at that deadline
- **THEN** the client MUST cancel that request at the overall deadline
- **AND** it MUST return a nonzero timeout outcome without waiting for the
  independent HTTP-client request timeout.

