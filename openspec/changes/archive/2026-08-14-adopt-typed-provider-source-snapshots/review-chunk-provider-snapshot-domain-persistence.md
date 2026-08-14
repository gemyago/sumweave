# Chunk Review: Provider Snapshot Domain And Persistence

## Initial State

- Scope: tasks 2.1-2.3.
- Status: predecessor chunk complete; implementation started after the ordered Enable Banking contract chunk.

## Implementation And Shallow Finalization

- Completed tasks 2.1-2.3 with a `ProviderSnapshot` domain model and validation for connection, account, and transaction attachments; supported semantic kinds; and credential-like document sanitization.
- Added the dedicated `finance_provider_snapshots` GORM schema and current-identity upsert store. A later capture replaces only the same tenant, connection, subject, finance-ID, kind, and provider-object identity; other kinds, subjects, and objects coexist.
- Added tenant-authorized account and transaction snapshot list/detail service methods. Lists remove the document and details sanitize it again as defense in depth.
- Wired connection-owned snapshot deletion through the existing `BankSyncService` metadata transaction, preserving its transaction-scoped store instance.
- Retained legacy evidence/raw-payload code only for the still-active connector and API flows. Task 6.1 owns its removal; no connector mapping or API route/controller work began in this chunk.

## Verification

- TDD red: `go test ./domain -run TestProviderSnapshot -count=1` initially failed to compile because `ProviderSnapshot` did not exist.
- TDD green: the domain test, dedicated-store identity test, and service/connection-cleanup test passed after implementation.
- Focused checks, finance module tests/lint, OpenSpec validation, and repository completion protocol are recorded in `tmp/crew-manager/adopt-typed-provider-source-snapshots-003-crew-p3-high-notes.md`.

## Result

- Status: complete; safe to begin the ordered `connector-mapping-sync-coordination` chunk (tasks 3.1-3.3).
- Commit: none; user explicitly requested no commit.

## Independent Review Finding Resolution

- The credential-like document sanitizer now also removes API/access keys, passwords, credentials, and passphrases. Domain, persistence, and detail-read regression coverage confirms those values are neither stored nor returned.
- Snapshot saves now confirm that the connection and finance account belong to the supplied tenant. Transaction snapshots also require the account to exist for that tenant and require the transaction to belong to that account.
- Domain validation now permits only `connection` on connection subjects, `account` or `account_balance` on account subjects, and `transaction` on transaction subjects.
- The focused tests, finance module test/lint, and final repository affected lint/test evidence are recorded in `tmp/crew-manager/adopt-typed-provider-source-snapshots-005-crew-p3-high-notes.md`.
- Resolution status: complete; the independent review no longer blocks chunk 3.
