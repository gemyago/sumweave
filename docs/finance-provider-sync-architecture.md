---
type: Architecture
title: "Finance provider sync"
description: "Link identity, bounded sync windows, conservative merge, and atomic progress."
sources:
  - id: bank-connection-service
    resource: "https://github.com/gemyago/sumweave/blob/99bac4b630a02983551c0775de946f8749a9d3b1/finance/bank_connection_service.go"
    title: "Current public bank-connection service boundary"
  - id: repository-policy
    resource: "repository docs/finance-provider-sync-architecture.md maintained procedures and conventions"
    title: "repository docs/finance-provider-sync-architecture.md maintained procedures and conventions"
  - id: documentation-snapshot
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/docs/finance-provider-sync-architecture.md"
    title: "Reviewed documentation checkpoint before this format migration"
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T12:06:29+00:00" }
---

# Finance Provider Sync Architecture

Provider sync imports normalized bank observations into finance records without
treating provider data as automatically correct ledger data.[^documentation-snapshot][^repository-policy]

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
while Enable Banking is its connector.[^documentation-snapshot]

The connector ID, provider reference, and encrypted connection-secret record
are durable connection metadata. A sync job uses them directly; it does not
choose a connector from product-provider-specific sync branches. Connectors
that need a credential resolve plaintext only through their bounded configured
resolver. Credentialless connectors use their configured credentials and the
durable provider reference.

The public `finance.BankConnectionService` enforces tenant membership and
maps public link requests to the internal coordinator. It receives the connector
and provider-profile registries through construction and uses the dedicated
`ProviderLinkPersistence` adapter. The public pending-start lookup supports the
Enable Banking callback bridge. App composition uses the public finance surface;
Go's `internal` boundary keeps coordinator types inside the finance module.[^bank-connection-service]

## Production Flow

Manual and scheduled bank-connection jobs share one path:[^documentation-snapshot]

```text
durable connection -> BankSyncService -> SyncOrchestrator
  -> target policy -> oldest-first requested windows -> WindowSyncExecutor
  -> connector fetch -> load existing window -> diff plan -> atomic apply
```

`BankSyncService` loads the connection and encrypted secret, records
start/success/failure and schedule diagnostics, and maps aggregate orchestration
statistics to the existing finance sync response and connection diagnostics.
The app worker owns the optional job projection; it exposes lifecycle and
sanitized error metadata only, not a generic job result or raw provider payload. The
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
a pure conservative diff plan, and applies it.[^documentation-snapshot]

The transactional apply creates a finance account and provider-account mapping
for a first observed provider account. It takes the tenant from the durable
connection, preserves member-edited account and transaction fields under the
merge rules, writes balances, matches, transactions, and typed provider
snapshots, and records created-account statistics accurately across chunks.

On success, all finance writes, the successful sync-state journal row, and the
`finance.bank-sync-window-completed.v1` event commit in the same transaction.
Classification and transfer-matching subscribers receive that window independently
and create no automatic job projection. Event-publication failure rolls back
that window alongside a ledger or journal failure. The row records the requested window, attempt and
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

## Related contracts

See [classification](product/classification.md), [transfer matching](product/transfer-matching.md),
and [background processing](architecture/background-processing.md) for downstream
reactions. The sync requirements are in this bundle; implementation links are
repository sources outside it:[^documentation-snapshot]

- [Finance sync scenarios](requirements/finance-management.md)
- [Window transaction implementation](../finance/internal/providers/window_sync_store.go)
- [Completion event adapter](../apps/sumweave/internal/financeapp/bank_sync_window_event.go)

Start a practical verification journey from the [manual guide index](manual-e2e/index.md).

[^documentation-snapshot]: Reviewed documentation checkpoint, preserved as the source of this section; format and navigation changes do not establish execution results.

[^repository-policy]: repository docs/finance-provider-sync-architecture.md maintained procedures and conventions.

[^bank-connection-service]: Current service construction, tenant checks, coordinator delegation, and pending-start lookup at the pinned source revision.
