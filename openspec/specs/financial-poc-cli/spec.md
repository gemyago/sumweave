# financial-poc-cli Specification

## Purpose
TBD - created by archiving change add-finance-poc-cli. Update Purpose after archive.
## Requirements
### Requirement: Financial POC CLI Command Tree
The backend application SHALL provide a lightweight financial POC command tree under the existing `sumweave` Cobra binary.

#### Scenario: Finance POC commands are discoverable under the existing binary
- **WHEN** a user runs the CLI command tree
- **THEN** `sumweave finance-poc` MUST expose `enable-banking` and `monobank` subcommands from `apps/sumweave/cmd/sumweave`
- **AND** the implementation MUST NOT require separate product services, UI routes, product database schemas, runtime slice changes, or app DI wiring

#### Scenario: JSON output is machine-readable
- **WHEN** a finance POC command is run with `--json`
- **THEN** stdout MUST contain machine-readable JSON with provider, operation, fetchedAt, summary, items where applicable, and raw provider payloads where applicable
- **AND** progress messages, authorization URLs, wait messages, and summary counts MUST be sent to stderr unless explicitly represented in the JSON payload

#### Scenario: Secrets are not exposed
- **WHEN** commands read tokens, private keys, JWTs, session files, or provider responses
- **THEN** the CLI MUST NOT hardcode secrets, MUST NOT log tokens/private keys/JWTs, MUST NOT commit live bank data, and MUST bound provider error response excerpts

#### Scenario: Fake servers can replace live providers
- **WHEN** commands or clients are tested offline
- **THEN** provider base URLs MUST be configurable by flag or env var and HTTP clients/timeouts MUST be injectable or otherwise fake-server-testable without live bank credentials

### Requirement: Enable Banking PKO POC Commands
The backend application SHALL provide Enable Banking POC commands for Polish ASPSP discovery, authorization/session creation, accounts, balances, and historical transactions.

#### Scenario: Polish ASPSPs can be listed
- **WHEN** a user runs `sumweave finance-poc enable-banking aspsps --country PL --json`
- **THEN** the command MUST authenticate with an RS256 JWT whose header includes `kid` equal to the application id, call `GET /aspsps?country=PL`, and print raw JSON or a JSON envelope containing the raw response

#### Scenario: Enable Banking session can be created with callback or manual code
- **WHEN** a user runs `sumweave finance-poc enable-banking start-auth --country PL --aspsp-name ... --psu-type ... --valid-days ... --redirect-url ... --auth-file ... --json`
- **THEN** the command MUST generate random state, call `POST /auth`, persist a pending auth JSON file before printing the authorization URL, and include provider, kind `pending-auth`, createdAt, state, request fields, normalized authorization URL, normalized provider auth/session identifier when present, and raw `POST /auth` payload needed by `finish-session`
- **AND** a manual `finish-session --auth-file ... --code ... --state ... --session-file ... --json` flow MUST load the pending auth file, verify state, call `POST /sessions`, and write a session JSON file containing provider, createdAt, ASPSP country/name, psuType, sessionId, access validity, accounts, and raw payload
- **AND** `connect --callback-listen-addr ... --session-file ... --json` MAY provide a one-process local-callback convenience flow with an HTTPS `https://.../callback` redirect URL, the same state verification, `POST /sessions` exchange, and session-file output
- **AND** if both `--callback-cert-file` and `--callback-key-file` are supplied, `connect` MUST load and use them for the local HTTPS listener
- **AND** if neither `--callback-cert-file` nor `--callback-key-file` is supplied, `connect` MUST use an ephemeral in-memory self-signed localhost/IP-SAN certificate for the local HTTPS listener
- **AND** if exactly one of `--callback-cert-file` or `--callback-key-file` is supplied, `connect` MUST fail clearly instead of silently falling back
- **AND** the command MUST NOT auto-open a browser unless `--open-browser` is explicitly provided

#### Scenario: Enable Banking accounts can be listed with optional details and balances
- **WHEN** a user runs `sumweave finance-poc enable-banking accounts --session-file ... --include-details --include-balances --json`
- **THEN** the command MUST read the session id, call `GET /sessions/{session_id}`, print accessible accounts, call `GET /accounts/{uid}/details` for each account when details are requested, call `GET /accounts/{uid}/balances` for each account when balances are requested, and include both thin normalized summaries and raw provider payloads

#### Scenario: Enable Banking transactions are paginated and exportable
- **WHEN** a user runs `sumweave finance-poc enable-banking transactions --session-file ... --account-id ... --from YYYY-MM-DD --to YYYY-MM-DD --json`
- **THEN** the command MUST call `GET /accounts/{account_id}/transactions` with `date_from` and `date_to`, pass `strategy` when provided, support `booked|pending|both` status mapping when straightforward, follow `continuation_key` until exhausted, preserve raw transaction payloads, include thin normalized transaction fields where easy, optionally write `--out`, and print transaction counts to stderr while keeping JSON stdout machine-readable

### Requirement: monobank Personal API POC Commands
The backend application SHALL provide monobank Personal API POC commands for account listing and historical statements.

#### Scenario: monobank accounts are listed from a personal token
- **WHEN** a user runs `sumweave finance-poc monobank accounts --json`
- **THEN** the command MUST read the personal token from `MONOBANK_TOKEN` or `--token`, call `GET /personal/client-info` with `X-Token`, and print normalized accounts, jars, and managed-client accounts with raw payloads
- **AND** normalized account output SHOULD include account id, type, currency code, balance, credit limit, masked PAN, and IBAN when present

#### Scenario: monobank transactions are chunked within provider limits
- **WHEN** a user runs `sumweave finance-poc monobank transactions --account 0 --from YYYY-MM-DD --to YYYY-MM-DD --json`
- **THEN** the command MUST convert dates to Unix seconds, split requests into chunks of at most 31 days plus 1 hour, call `GET /personal/statement/{account}/{from}/{to}` for each chunk, respect a safe default inter-request sleep such as `61s`, allow sleep override for tests, preserve raw statement items, include thin normalized transaction fields where easy, optionally write `--out`, and print transaction counts to stderr while keeping JSON stdout machine-readable

### Requirement: Financial POC Documentation And Offline Tests
The repository SHALL document manual setup and provide offline coverage for the financial POC commands.

#### Scenario: Enable Banking PKO setup is documented
- **WHEN** a user reads `apps/sumweave/doc/financial-poc/enable-banking-pko.md`
- **THEN** the doc MUST explain Enable Banking sandbox and production application registration, private key storage outside the repo, application id as JWT `kid`, HTTPS local callback URL with trusted certificate guidance, callback cert/key pairing requirements, self-signed fallback limitations, tunnel/manual fallback, restricted production account linking, required and optional env vars, ASPSP discovery for PKO, live AI-agent test commands, and the fact that an AI agent cannot complete PKO strong customer authentication

#### Scenario: monobank setup and limits are documented
- **WHEN** a user reads `apps/sumweave/doc/financial-poc/monobank.md`
- **THEN** the doc MUST explain personal token generation, secret storage outside the repo, required env vars, account listing, statement commands, AI-agent live test commands/steps for listing accounts and fetching a bounded statement range, 60-second endpoint limits, 31-day-plus-1-hour statement range cap, default account `0`, and warning that repeated live calls can hit monobank rate limits

#### Scenario: Default tests do not require live bank credentials
- **WHEN** module tests run in CI or locally without provider secrets
- **THEN** tests MUST use fake HTTP servers/generated non-secret keys and MUST cover Enable Banking JWT creation, Enable Banking endpoints and transaction pagination, monobank `X-Token` requests, monobank date range chunking, and short/no sleep command execution without calling live bank APIs

#### Scenario: Financial POC artifacts are ignored
- **WHEN** session files or transaction exports are created using documented paths or filename patterns
- **THEN** `.gitignore` MUST cover `data/financial-poc/`, `*.enable-banking-session.json`, `*.monobank-transactions.json`, and `*.enable-banking-transactions.json` either directly or through broader existing ignore rules

