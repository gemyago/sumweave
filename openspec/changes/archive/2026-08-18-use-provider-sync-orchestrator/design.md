## Context

The finance module contains two sync execution paths. The production durable job calls `BankSyncService.RunBankConnectionSync`, which chooses a connector through a legacy product-provider mapping, fetches one broad window through a connector-backed `BankConnectionProvider`, converts the observation batch into legacy result types, and applies it through service-owned persistence logic. Separately, `finance/internal/providers` contains the intended `SyncOrchestrator`, target-window policy, requested-window executor, pure diff/apply planners, provider-owned window sync store, and append-only state journal.

The production path therefore uses v2 connectors but not v2 orchestration. The dormant path also has runtime-readiness gaps: there is no concrete chunk policy, explicit job window bounds do not reach target planning, initial account mappings cannot be created by the window store, tenant identity for a new transaction is inferred from existing transactions, and a successful apply commits before its success state is appended. The legacy adapter also decrypts credentials and repackages plaintext as an envelope, while the v2 Monobank connector already has a bounded secret resolver seam and the credentialless Enable Banking fetch currently rejects an otherwise valid persisted secret record solely because its identity fields are populated.

The finance module must continue to own all provider composition behind `finance.New`. The app must continue to depend only on focused public finance services. Durable job delivery is at least once, persisted connection secrets remain encrypted at rest, and the existing HTTP and job contracts must remain unchanged.

## Goals / Non-Goals

**Goals:**

- Make the existing provider sync orchestrator the only production execution path for manual and scheduled bank sync jobs.
- Resolve product provider and technical connector identity from the durable bank connection without provider-specific sync branches.
- Make journal state authoritative for automatic coverage planning and failed-attempt checkpoint derivation.
- Preserve optional explicit window bounds and split every resolved target into oldest-first windows of at most 30 calendar days.
- Make the requested-window store able to create the first linked finance accounts, provider mappings, transactions, balances, matches, and typed snapshots.
- Preserve accurate imported-account job statistics across multi-chunk execution.
- Commit successful requested-window writes and their journal checkpoint atomically.
- Preserve scheduling, connection diagnostics, conservative matching, user edits, and current external contracts.
- Delete the legacy connector adapter and service-owned fetch/apply path after cutover.

**Non-Goals:**

- Change bank-linking behavior or the `LinkCoordinator` path.
- Add provider discovery, new providers, new APIs, or new UI.
- Add a provider snapshot history timeline or retain successful raw response bodies.
- Add a compatibility toggle or preserve internal Go APIs used only by the legacy sync path.
- Redesign the durable jobs framework.

## Decisions

1. Keep job lifecycle ownership in `BankSyncService` and inject one narrow orchestration dependency.

   `BankSyncService` remains responsible for trigger authorization, schedule operations, loading the durable connection and secret, marking connection/schedule start and completion state, mapping orchestration stats to `BankConnectionSyncResult`, and recording a whole-job failure. Its execution dependency is a consumer-defined interface exposing only orchestration of one connection sync request.

   `finance.New` constructs the concrete journal, target policy, chunk policy, window persistence adapter, provider-owned window store, executor, and orchestrator, then supplies the orchestrator to the focused bank-sync service. The app continues to receive only `*finance.BankSyncService`.

   An adapter that translates orchestration output back into `ProviderSyncResult` is rejected because the requested-window executor already applies finance writes. Feeding that result into the legacy apply path would duplicate persistence and retain two competing orchestration models.

2. Construct sync identity only from durable connection metadata.

   The service builds `domain.ProviderConnectionRef` from `BankConnection.ID`, `Provider`, `ConnectorID`, and `ProviderReference`. The executor resolves `ConnectorID` through the connector registry. Product provider `pko` therefore remains distinct from connector `enable-banking`, and empty or unknown persisted connectors fail before provider fetch.

   The service loads the stored encrypted `domain.ConnectionSecret` through a narrow reader and passes that record unchanged to orchestration. Connector composition injects the existing cipher-backed token resolver where a connector, currently Monobank, needs plaintext. A credentialless connector ignores the record and uses its own configured credentials plus the durable provider reference; Enable Banking must no longer reject a valid record merely because its ID or reference is populated. Decryption stays in the bounded resolver, and neither plaintext nor the encrypted envelope is logged or copied into snapshots.

3. Put explicit bounds and automatic checkpoint planning behind one target-window policy.

   The orchestration request carries optional `WindowStart` and `WindowEnd` values from the existing job input. Target planning first resolves the end to the provided value or the orchestration clock. A provided start is authoritative; otherwise the policy derives the start from the latest journal state relative to that resolved end using the existing first-sync, succeeded-checkpoint, failed-attempt, and recent-refresh rules. Both explicit bounds therefore bypass journal-derived bounds, a start-only request ends at the orchestration clock, and an end-only request derives its start relative to that end. The final target is always validated. This makes the journal, rather than `BankConnection.LastSuccessfulSyncAt`, the automatic planning source of truth.

   A concrete calendar-day chunk policy validates the resolved target and returns contiguous half-open windows in ascending order. Each end is the earlier of the target end and `start.AddDate(0, 0, 30)`, preserving the input location without explicit timezone normalization. Provider-specific pagination or transport limits may still exist inside connectors, but they do not replace orchestration chunks.

4. Make connection ownership the source for first-sync persistence.

   `ProviderWindowSyncStore.ApplySync` loads the durable bank connection once inside its transaction. For an unseen provider account it creates a linked finance account owned by the connection tenant, including product-provider linkage, and then creates its `ConnectionProviderAccount` mapping before applying balances, transactions, and snapshots. Existing mappings continue to refresh provider metadata while preserving member-edited account fields under the existing account refresh rules.

   New transactions receive `TenantID` from the durable connection. The current inference from an existing transaction in the snapshot is removed because it prevents an account's first transaction and couples ownership to incidental data.

   The diff/apply result also distinguishes created accounts from merely observed accounts. `ProviderSyncStats` and its journal model gain a created-account count so `BankConnectionSyncResult.ImportedAccounts` is not inflated by observing the same account in every chunk. These operations remain behind provider-owned, consumer-defined persistence interfaces. The change does not add new orchestration methods to the legacy broad `persistence.Store`; dedicated adapters expose the required transaction operations.

5. Commit successful chunk state in the same transaction as chunk writes.

   The executor assigns the chunk run identity before apply and builds the completed state from the attempt metadata, completion time, window, job ID, and aggregate stats through the current chunk. The window store accepts that successful completion state and appends it through the same finance database transaction that applies accounts, balances, transactions, matches, and snapshots. If either the writes or journal append fails, the entire requested window rolls back and the executor returns an error.

   Failed attempts have no successful finance apply to coordinate. After rollback or a fetch/planning failure, the orchestrator appends a failed state through the standalone journal seam. The next automatic plan derives its checkpoint from the failed window start and then applies the existing recent-refresh rule; it need not start exactly at the failed bound when that bound is recent.

   This changes the current division where the orchestrator appends both outcomes after the executor returns. Atomic success completion is preferred over preserving that separation because a committed window without its checkpoint would be indistinguishable from an unexecuted window during retry.

6. Keep mutable connection and schedule timestamps as operational projections.

   `LastSyncStartedAt`, `LastSuccessfulSyncAt`, `LastSyncError`, job IDs, reauthorization state, and schedule timestamps remain available to current APIs and UI. They describe the whole durable job and are updated around orchestration. They do not decide automatic coverage.

   Earlier chunks may commit before a later chunk fails. In that case the job and connection expose failure while the journal retains exact successful and failed chunk progress. The next job resumes from journal state instead of discarding completed coverage.

   Connection deletion also removes its provider sync state journal through a dedicated cleanup dependency rebound to the existing connection-cleanup transaction, so a later cleanup failure rolls the journal deletion back and no connection-owned orchestration records remain orphaned after success.

7. Cut over directly and remove the legacy path.

   Once focused orchestration and persistence tests pass, remove `connectorBankSyncProvider`, `BankConnectionProvider.Sync`, `ProviderSyncParams`, `ProviderSyncResult`, result conversion, `ApplyProviderSyncResult`, the legacy product-provider sync selector, and persistence methods used only by that path. Provider linking methods remain on their existing focused v2 service and connector contracts. Align `docs/finance-provider-sync-architecture.md` with the atomic success flow and remove the obsolete executor coordination plan once it no longer describes remaining work.

   A runtime toggle or fallback is rejected because it would leave two sources of sync behavior and allow the persisted connector requirement to be bypassed.

8. Verify the production composition boundary, not only isolated components.

   Focused tests cover target and chunk policies, atomic success/failure journaling, initial account creation, tenant derivation, accurate result statistics, connector identity, encrypted-secret handoff, and service lifecycle projections. A SQLite-backed `finance.New` composition test starts from `BankSyncService.RunBankConnectionSync` and proves a first synthetic sync uses the production stack. Controlled orchestrator/service tests cover PKO/Enable Banking identity, explicit bounds, a failed middle chunk, durable earlier progress, and the next journal-derived plan without requiring PostgreSQL. The app job-handler test verifies that the unchanged durable job input still reaches the focused service.

## Risks / Trade-offs

- [Atomic journal completion broadens the apply transaction contract] -> Keep the interface provider-sync-specific and expose journal append only through the dedicated transaction adapter.
- [A later chunk can fail after earlier chunks committed] -> Treat chunks as independent durable progress, record the failed attempt, and resume from the journal checkpoint.
- [Explicit windows can overlap previously completed coverage] -> Continue conservative diff/match behavior and treat operator-supplied bounds as an intentional refresh request.
- [Removing legacy declarations may require broad test cleanup] -> Delete tests that assert the obsolete path and replace them with focused orchestration and production-composition behavior tests.
- [Connection status and journal state can temporarily describe different scopes] -> Document that connection fields describe the whole job while journal rows describe individual attempted windows.

## Migration Plan

1. Extend orchestration requests and implement the concrete target/chunk planning behavior with focused tests.
2. Make initial account/transaction persistence and created-account statistics work with a real finance database.
3. Make successful state completion part of atomic requested-window apply and add transactional journal cleanup.
4. Add the narrow orchestration dependency to `BankSyncService`, construct the full stack in `finance.New`, and move production execution to it while retaining current lifecycle projections and external contracts.
5. Add production-composition and durable-handler tests, including failure/resume and first-sync scenarios.
6. Remove the legacy adapter, result DTOs, apply path, selector branches, obsolete tests, and stale coordination plan; then update the provider sync architecture document and run repository completion checks.

The schema changes are additive or affect unused early-alpha state, so no compatibility rollout or dual-write phase is required. Rollback is a source revert before relying on newly written journal progress; no runtime fallback path will be retained.

## Manual Verification

After all implementation chunks and automated checks are green, run the existing `docs/manual-e2e/enable-banking-mock-aspsp-ui-e2e.md` flow in a headed Playwright browser. Verify PKO remains the product provider, Enable Banking remains the technical connector, the unchanged sync job completes, and linked accounts, transactions, and provider source data appear. This is a final verification activity, not a separate implementation task; environment or interactive authorization blockers must be reported rather than bypassed.

## Open Questions

None. The existing API bounds, 30-day policy, append-only journal model, connector registry, and early-alpha compatibility stance provide the required decisions.
