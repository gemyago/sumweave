## MODIFIED Requirements

### Requirement: Finance Sync, Secrets, And Imports
The finance module SHALL support secure provider linking plus explicit async sync/import workflows.

#### Scenario: Provider sync v2 requested-window execution plans before apply handoff
- **WHEN** the window sync executor receives a fetched provider sync batch and a loaded existing snapshot
- **THEN** it MUST build a `ProviderDiffPlan`
- **AND** it MUST build an `ApplyPlan` from that diff plan before requesting persistence writes
- **AND** it MUST hand the resulting plans to an executor-facing `WindowSyncStore` instead of persisting ledger writes directly inside connector code

#### Scenario: Provider sync v2 window sync store owns workflow coordination
- **WHEN** provider sync v2 implements the requested-window storage seam used by the window sync executor
- **THEN** that seam MUST be named `WindowSyncStore`
- **AND** the concrete `WindowSyncStore` MUST live in `finance/internal/providers`
- **AND** it MUST depend on narrow consumer-defined snapshot-read and transactional-apply persistence interfaces rather than a broad concrete persistence store
- **AND** it MUST coordinate snapshot loading and apply-plan persistence without deciding transaction matching strategy, merge policy, stats, or issues

#### Scenario: Provider sync v2 apply reuses canonical persistence operations
- **WHEN** provider sync v2 applies a planned requested-window sync
- **THEN** it MUST reuse existing canonical persistence methods for entity saves when their semantics match the apply need
- **AND** provider transaction writes MUST use the canonical transaction save path instead of duplicating transaction upsert behavior
- **AND** the apply workflow MUST run provider-account, balance, raw-payload, transaction, and provider-transaction-match writes inside one transaction boundary

#### Scenario: Provider sync v2 snapshot loading uses sync-shaped persistence queries
- **WHEN** provider sync v2 loads persisted comparison data for a snapshot lookup window
- **THEN** it MUST load connection provider accounts for the connection first
- **AND** it MUST load provider-source transactions for the mapped finance accounts whose effective time falls inside the snapshot window
- **AND** it MUST load provider transaction matches for the same connection whose `TransactionID` belongs to that loaded transaction set
- **AND** it MUST NOT overload user-facing transaction list queries when their filters, ordering, or visibility semantics do not match snapshot loading
