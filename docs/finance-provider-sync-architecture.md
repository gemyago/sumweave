---
type: Architecture
title: "Finance provider sync"
description: "Bank authorization, retained identity and history, bounded sync windows, and atomic progress."
sources:
  - id: replay-delivery
    resource: "repository finance/internal/providers/sync_orchestrator.go, finance/service_bank_sync.go and apps/sumweave/internal/appdispatch/appdispatch.go"
    title: "Committed replay progress and durable delivery lifetime"
  - id: session-recovery
    resource: "repository finance/bank_authorization_service.go and finance/domain/bank_authorization.go"
    title: "Existing-schema bank link replacement and replay implementation"
  - id: bank-connection-service
    resource: "https://github.com/gemyago/sumweave/blob/99bac4b630a02983551c0775de946f8749a9d3b1/finance/bank_connection_service.go"
    title: "Current public bank-connection service boundary"
  - id: repository-policy
    resource: "repository docs/finance-provider-sync-architecture.md maintained procedures and conventions"
    title: "repository docs/finance-provider-sync-architecture.md maintained procedures and conventions"
  - id: documentation-snapshot
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/docs/finance-provider-sync-architecture.md"
    title: "Reviewed documentation checkpoint before this format migration"
generated: { by: "codex/gpt-6", at: "2026-11-27T17:25:03+01:00" }
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

## Updating Enable Banking authorization

An active or blocked connection offers **Update bank link**. Start binds the
pending JSON to the existing connection and expected session, tenant, actor,
provider and connector. Finish consumes that state once within its TTL and checks
the expected session before exchange and again under the connection row lock.
A cancelled or failed update leaves the installed authorization unchanged. The
public callback only hands allowlisted code/state/error fields to the stored,
validated browser callback; it rejects expired or consumed state.[^session-recovery]

No SQL columns or tables are added. Existing encrypted secret payloads hold a
versioned application authorization document: bank name/country and account
bindings with primary hash, canonical account key, current remote UID, currency,
selection and optional replay floor. Exact unique primary hashes preserve keys
across session changes. Missing identities, conflicting currency, bank changes
and key collisions fail safely. Legacy empty secrets bootstrap from the exact
saved current-session snapshot and each mapped account's original source snapshot,
with tenant and account attachment checks. Raw provider snapshots stay truthful;
they do not contain application binding metadata.[^session-recovery]

Replacement atomically installs the secret, session and source snapshot on the
same connection. It retains the name, account mappings, ledger IDs and history,
and schedule preferences. Deselected accounts remain visible in the ledger and
are omitted from bank fetches. Re-addition reuses their original canonical keys.
Canonicalization occurs before transaction fallback IDs and fingerprints are
computed. Existing transaction matching and its ambiguity limits still
apply.[^session-recovery]

Added accounts start at the intended three-year initial history floor.
Deselection retains the earlier unfinished replay floor or normal policy start
at replacement. Selected floors widen the existing shared target window, so
other selected accounts can be fetched again across that history. Only a fully
successful unbounded run clears covered selected floors. Each committed chunk
advances a covered selected floor to its end, only when it covers that floor;
a gap cannot be skipped. Progress is saved after the ledger/checkpoint transaction
under the expected-session fence. A crash between those commits can repeat the
last chunk, but cannot skip uncommitted history. Partial failures retain the
advanced floors for retry. Bounded requests and inactive accounts do not change
floors; later reselection retains its earlier catch-up requirement.[^replay-delivery]
An addition or reselection publishes an observed unbounded sync command through
appdispatch in the same transaction as replacement; schedule preference and due
occurrence stay unchanged. A further update with outstanding selected replay also
queues that work, because replacement fences an older in-flight attempt. Bank-side history availability remains a provider
limit. The returned connection includes the actually published job ID, saved in
the replacement transaction. The initiating connection card tracks that job
immediately through the existing queued/running/pending feedback.[^session-recovery]

Durable delivery has no implicit 30-second transport acknowledgement deadline.
A handler runs its finite planned windows with the worker's cancellation context;
provider HTTP requests retain their own timeouts. Shutdown cancels delivery and
leaves unacknowledged work for retry. Ordinary operation failures use the existing
bounded router retries and durable dead-letter path with a live parent context.
The PostgreSQL subscriber keeps its transaction open until acknowledgement, so
long deliveries can delay snapshot/vacuum progress and other messages on that
subscription. This change adds no per-account scheduler or continuation commands;
committed floors make interrupted shared replay resumable.[^replay-delivery]

Explicit CLOSED/EXPIRED session states or CLOSED_SESSION/EXPIRED_SESSION response
codes persist `reauth_required` with short feedback and terminal job failure.
A plain HTTP 401 does not establish session closure. Manual, queued and scheduled
work stops while blocked; schedules remain enabled and due. Replacement, ledger
apply, failed journal appends and sync projections lock the same connection row.
Expected-session and active-state fences reject late writes from old sessions.
Worker timing updates cannot rewind due time or overwrite schedule preferences.
See the [acceptance scenarios](requirements/finance-management.md#requirement-bank-link-update-and-session-recovery).[^session-recovery]

[^session-recovery]: Existing-schema authorization replacement, retained identity and replay implementation in `finance/bank_authorization_service.go`, `finance/domain/bank_authorization.go`, provider sync persistence and the Finance connection card.

[^replay-delivery]: Replay callbacks follow successful window apply; encrypted progress is session-fenced in the authorization service. Appdispatch explicitly disables the Watermill SQL subscriber acknowledgement deadline; per-request HTTP timeouts and worker cancellation remain active.
