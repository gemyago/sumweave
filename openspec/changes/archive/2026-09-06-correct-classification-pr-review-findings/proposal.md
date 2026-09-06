## Why

PR #13 review found three correctness gaps in the completed Phase 0 transaction
classification flow: transient category reads are acknowledged as terminal,
classification tenant denials become server errors, and a replacement manual-rule
offer can retain stale form defaults. These focused corrections are needed before
the submitted change can be accepted.

## What Changes

- Keep actual unavailable classification-category states terminal, while returning
  transient category lookup failures as ordinary errors so appdispatch can retry
  and dead-letter them normally.
- Map an explicit classification submission's tenant access denial to the standard
  empty `401 Unauthorized` finance response.
- Refresh optional manual-rule form defaults when a new offer replaces the current
  offer, while preserving operator edits for the lifetime of the same offer.
- Add focused regressions for each corrected boundary and verify each corresponding
  PR #13 review thread after implementation.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `transaction-classification`: Clarify retryable category lookup failures and the
  explicit classification tenant-denial HTTP contract.
- `finance-operator-ui`: Clarify draft refresh and edit preservation when a
  manual-assignment rule offer is replaced.

## Impact

- Finance classification execution and its focused service tests.
- Backend finance controller error mapping and registered-route tests.
- Optional rule-creation form/owners, focused Svelte tests, and the Finance UI
  wireframe's offer-lifetime behavior note.
- No schema, endpoint shape, generated API, dependency, or migration change.
