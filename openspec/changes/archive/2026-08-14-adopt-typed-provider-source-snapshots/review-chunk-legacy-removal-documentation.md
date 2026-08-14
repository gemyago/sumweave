# Chunk Review: Legacy Removal And Documentation

## Implementation — 2026-08-14

- Scope: task 6.1 code and documentation portion.
- Removed the retired provider-evidence/raw-payload domain, stores, schema
  models, auto-migration registration and compaction, read/write seams,
  connector response retention, generated mocks, and obsolete tests.
- Regression coverage now verifies the clean finance migration does not create
  either retired table. Registered-route coverage continues to require account
  and transaction `/evidence` paths to return `404`.
- Updated active terminology, architecture, database-recreation/upgrade-cleanup,
  and Mock ASPSP manual-E2E guidance for current typed provider snapshots.
- `review-final.md` contains the required operator SQL handoff block.

## Verification

- Focused finance migration/snapshot and protected-route checks passed.
- Finance, Sumweave, and UI module lint/test suites passed.
- Repository `make affected-lint-test` passed.
- Strict OpenSpec validation passed.

## Pending final acceptance

The user requested Mock ASPSP manual E2E as a separate final invocation. It is
pending manager acceptance and must be run on a recreated local database before
whole-change review/release handoff. This does not reopen the completed 6.1
implementation/docs task.

## Whole-change Review Remediation — 2026-08-14

- The pending redirect-start persistence path now sanitizes and re-encodes its
  typed document before saving. Invalid JSON prevents any pending-start write.
- Focused regression coverage verifies nested credential-like values are
  removed and no redirect-start document becomes a current provider snapshot.
- The stale public `ProviderRawPayload` DTO note was removed from the active
  v2 boundary documentation.
- Finance lint/test, repository `make affected-lint-test`, and strict OpenSpec
  validation passed. Manual Mock ASPSP E2E remains deferred.
