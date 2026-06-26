## 1. Domain Foundation

- [x] 1.1 Define provider identity and observation domain types, and must follow TDD flow by first writing failing compile-focused or unit tests that exercise `ProviderID`, `ProviderConnectorID`, provider connection references, account observations, balance observations, transaction observations, raw payload observations, and provider sync batches before implementing and verifying focused tests.
- [x] 1.2 Define provider sync state, run, stats, and issue domain types, and must follow TDD flow by first writing failing compile-focused or unit tests that construct per-connection sync state, requested/candidate windows, run status, aggregate stats, and sync issues before implementing and verifying focused tests.

## 2. Internal Provider Contracts

- [x] 2.1 Add `finance/internal/providers` connector and profile contracts, and must follow TDD flow by first writing failing compile-focused or unit tests proving the connector interface supports linking, fetching, capabilities, and provider profile composition where PKO uses the Enable Banking connector before implementing documented stubs and verifying focused tests.
- [x] 2.2 Add provider fetch and snapshot contracts, and must follow TDD flow by first writing failing compile-focused or unit tests proving fetch requests carry connection, secret, window, and optional sync state while existing-window snapshots carry persisted provider accounts, transactions, and matches before implementing documented stubs and verifying focused tests.

## 3. Diff And Apply Planning

- [x] 3.1 Add the pure diff planner and diff plan data structures, and must follow TDD flow by first writing failing unit tests proving strong provider-ID matches update, fingerprint matches update, weak or ambiguous matches create new transaction actions, and stats count ambiguous-created transactions before implementing stubs and verifying focused tests.
- [x] 3.2 Add apply planner and merge-policy stubs, and must follow TDD flow by first writing failing unit tests proving comments and placeholder behavior preserve the intended contract that provider-original fields refresh while user-facing fields are preserved when they differ from previous provider-original values before implementing documented stubs and verifying focused tests.

## 4. Sync Coordinator Skeleton

- [x] 4.1 Add the sync coordinator skeleton, and must follow TDD flow by first writing failing compile-focused or unit tests proving coordinator requests include connection, secret, requested window, job ID, and reason, and coordinator results expose run ID, stats, and issues before implementing documented stubs and verifying focused tests.
- [x] 4.2 Keep v1 provider sync behavior unchanged, and must follow TDD flow by first running focused existing finance provider-sync tests or adding a failing guard where needed before confirming no v2 contract wiring changes existing monobank or PKO sync execution.
