## 1. Journal Persistence Model

- [x] 1.1 Add the provider sync state journal persistence model and migration registration, and must follow TDD flow by first writing failing store-focused tests proving the finance schema can persist append-only succeeded-state rows with nullable successful-window coverage, aggregate stats columns, and UTC timestamp normalization before implementing and verifying focused tests.
- [x] 1.2 Add a dedicated journal store component with latest-succeeded load behavior, and must follow TDD flow by first writing failing store-focused tests proving an empty journal returns no state, lookup is scoped to one connection, and the newest appended snapshot is returned without mutating older rows before implementing and verifying focused tests.

## 2. Orchestrator Seam Compatibility

- [x] 2.1 Add dedicated journal store append/load methods that round-trip `domain.ProviderSyncState` losslessly, and must follow TDD flow by first writing failing focused tests proving attempt time, success time, run/job identity, error summary, and aggregate stats survive append/load mapping before implementing and verifying focused tests.
- [x] 2.2 Confirm the dedicated finance journal store satisfies the existing sync orchestrator journal seam, and must follow TDD flow by first writing failing compile-focused or unit tests proving the journal store can be used as `internal/providers.SyncStateJournal` and that load/append database errors are wrapped with journal-specific context before implementing and verifying focused tests.
