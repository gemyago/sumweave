# Finance Provider Sync Architecture

Provider sync imports normalized bank observations into finance records without
treating provider data as automatically correct ledger data.

## Terms

- Product provider: the bank brand a member selects, such as PKO or monobank.
- Connector: the technical integration used to access a provider, such as
  Enable Banking.
- Connection: one durable linked bank access record for a tenant.
- Target window: the overall coverage range for one sync job.
- Requested window: one half-open chunk within the target window.
- Provider snapshot: the current sanitized, schema-derived provider document
  for a connection, account, balance, or transaction. It is not a raw HTTP
  response or a historical response archive.
- Sync-state journal: the append-only history of requested-window attempts for
  one connection.

## Linking And Identity

Provider sync v2 owns bank linking through the `LinkCoordinator`. Linking
resolves the member-facing product provider to a connector and persists both
identities on the connection. PKO, for example, remains the product provider
while Enable Banking is its connector.

The connector ID, provider reference, and encrypted connection-secret record
are durable connection metadata. A sync job uses them directly; it does not
choose a connector from product-provider-specific sync branches. Connectors
that need a credential resolve plaintext only through their bounded configured
resolver. Credentialless connectors use their configured credentials and the
durable provider reference.

## Production Flow

Manual and scheduled bank-connection jobs share one path:

```text
durable connection -> BankSyncService -> SyncOrchestrator
  -> target policy -> oldest-first requested windows -> WindowSyncExecutor
  -> connector fetch -> load existing window -> diff plan -> atomic apply
```

`BankSyncService` owns job lifecycle projections: it loads the connection and
encrypted secret, records start/success/failure and schedule diagnostics, and
maps aggregate orchestration statistics to the existing job result. The
orchestrator and executor own coverage, connector fetching, planning, and
requested-window persistence.

The orchestrator preserves explicit job bounds. A supplied start or end is
used unchanged; an omitted end is the orchestration clock and an omitted start
comes from journal policy relative to that end. Automatic planning uses the
latest journal state, not `LastSuccessfulSyncAt`. The resulting target is
validated, then split into contiguous half-open requested windows of at most
30 calendar days and executed oldest first without explicit timezone
normalization.

## Requested-Window Apply

For each requested window, the executor resolves the persisted connector,
fetches normalized observations, loads the matching persisted window, creates
a pure conservative diff plan, and applies it.

The transactional apply creates a finance account and provider-account mapping
for a first observed provider account. It takes the tenant from the durable
connection, preserves member-edited account and transaction fields under the
merge rules, writes balances, matches, transactions, and typed provider
snapshots, and records created-account statistics accurately across chunks.

On success, all finance writes and the successful sync-state journal row commit
in the same transaction. The row records the requested window, attempt and
success time, run/job identity, and aggregate statistics. A journal-write or
finance-write failure rolls back that requested window.

Fetch, planning, or apply failures leave no partial writes for their requested
window. The orchestrator then appends a failed journal attempt through its
standalone journal path. Earlier successful chunks remain durable when a later
chunk fails, and the next automatic target derives its checkpoint from the
latest failed or successful journal state. At-least-once delivery can therefore
resume without refetching completed chunks or duplicating their writes.

Deleting a connection removes its journal records in the existing metadata
cleanup transaction, together with connection-owned provider data.

## Design Principles

- Keep product-provider identity distinct from technical connector identity.
- Keep encrypted secrets at rest; never log, persist, or snapshot plaintext.
- Keep provider observations and typed snapshots separate from ledger data.
- Plan before writing and use conservative transaction matching.
- Preserve member edits under the existing merge rules.
- Treat journal rows as durable per-window progress and connection fields as
  whole-job operational projections.
