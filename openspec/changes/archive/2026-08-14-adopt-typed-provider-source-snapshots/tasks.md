## 1. Enable Banking Typed Contract

- [x] 1.1 Add documentation-derived fixtures covering every documented supported session-account, account-details, account-balance, transaction-page, and transaction-item field from the official reference checked on August 14, 2026; follow TDD by first adding semantic decode/encode round-trip tests that fail against the partial DTO graph, then verify zero, false, empty, nested, and continuation values survive and record the reference URL/date in fixture comments.
- [x] 1.2 Complete the Enable Banking response DTO graph without raw maps, raw messages, or successful response-body fields; follow TDD by implementing only enough typed fields and optional representations to pass the full-contract fixtures, then run the finance module tests and lint for the changed client packages.

## 2. Provider Snapshot Domain And Persistence

- [x] 2.1 Introduce the `ProviderSnapshot` domain model, finance subjects, supported snapshot kinds, and schema-derived document terminology; follow TDD by first specifying required common fields plus the connection/account/transaction finance-ID combinations, stable provider-object identity, sanitization, and serialization behavior, then implement the smallest domain surface that satisfies those tests.
- [x] 2.2 Add the dedicated current provider-snapshot GORM store and schema; follow TDD by first testing replacement for an identical identity and coexistence across subject, provider object, and kind, then implement auto-migration and store operations without compatibility copying from legacy tables.
- [x] 2.3 Replace provider-evidence and raw-payload write/read services with tenant-authorized provider-snapshot services; follow TDD by first covering account and transaction ownership, list/detail metadata, latest-only reads, defense-in-depth sanitization, and connection-owned snapshot deletion, then implement the service transaction boundaries.

## 3. Connector Mapping And Sync Coordination

- [x] 3.1 Refactor Enable Banking connector normalization to leave provider DTOs unchanged and emit `connection`, `account`, `account_balance`, and per-item `transaction` snapshots; follow TDD by first covering normalized finance values alongside semantic DTO snapshot equality and the absence of transaction-page snapshots, then implement the mapping.
- [x] 3.2 Adapt Monobank and synthetic connectors to encode their typed provider-owned source structures instead of forwarding raw JSON or generic source maps; follow TDD by first asserting connection/account/per-item transaction kinds, provider-object identities, sanitized documents, normalized observations, no transaction-page snapshots, and no duplicated `account_balance` kind when no distinct typed balance document exists, then implement the common snapshot output.
- [x] 3.3 Integrate provider snapshots into link-finish and both active bank-sync and provider-window-sync persistence while removing provider-evidence/raw-payload writes; follow TDD by first covering atomic connection/secret/final-snapshot persistence, typed and secret-safe pending-start documents that never enter current-snapshot storage, atomic normalized-record/match/snapshot apply, idempotent replacement, distinct Enable Banking account/balance documents, per-transaction attachment, and secret-safe failure behavior, then implement the orchestration changes without extending the legacy broad store.

## 4. Protected Finance API

- [x] 4.1 Replace account and transaction `/evidence` contracts with `/provider-snapshots` metadata and detail contracts in the backend OpenAPI surface, using `data` for the optional detail document; follow TDD by first updating route and response contract tests, then regenerate and implement focused controllers without compatibility routes or aliases.
- [x] 4.2 Wire provider-snapshot authorization and response sanitization through the Sumweave application; follow TDD by first covering tenant membership, cross-tenant denial, missing snapshot behavior, and source-document responses, then implement application composition and run the affected backend tests and lint.

## 5. Finance Operator UI

- [x] 5.1 Replace evidence API mappings and account/transaction disclosure state with provider-snapshot mappings; follow TDD by first updating UI API and component tests for lazy metadata/detail loading, kind rows, empty/error recovery, and no history affordance, then implement the state and generated-client integration.
- [x] 5.2 Update affected UI components and wireframes to use “Provider source data” and explain its schema-derived latest-snapshot semantics; follow the visual verification flow by adjusting the design, checking account and transaction details at supported responsive widths, and resolving visible hierarchy, copy, loading, empty, and error issues before running the UI tests and lint.

## 6. Legacy Removal And Documentation

- [x] 6.1 Remove obsolete provider-evidence/raw-payload domain, persistence, API, UI, and connector paths and update finance terminology and architecture documentation; follow TDD by first updating affected regression tests to reject legacy routes and writes, then delete unused code, document database recreation and manual cleanup, reproduce the design's exact **Operator action required after upgrade** block in `review-final.md` for later pull-request/release handoff, complete the repository `make affected-lint-test` protocol, and finish the completion protocol with the configured Mock ASPSP runbook on a recreated local database, verifying successful PKO link/sync, distinct account and account-balance source data, one complete transaction-item snapshot, and absent protected `/evidence` routes.
