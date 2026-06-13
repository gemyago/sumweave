## Context

The current data layer persists canonical instruments, candles, and trades with source provenance, deterministic reads, and replay identities. That is enough for normalized market-data consumption, but it does not preserve the raw venue response or the normalization/batch context that produced a canonical row.

The repository architecture makes `runtime/data` the owner of normalization, quality state, and replayable persistence, while venue-specific mechanics stay at the venue edge. This change should therefore add a small data-layer lineage model that can be populated by current or future ingestion flows without creating a generic venue framework, scheduler, backend API, or UI.

## Goals / Non-Goals

**Goals:**

- Add minimal lineage records for `IngestionRun`, `RawVenuePayload`, `NormalizationRun`, and `DataBatch` in the data layer.
- Store raw payload bytes or text plus non-sensitive source metadata in the data-layer database for replay/audit use.
- Persist normalization and batch metadata with UTC timestamps, explicit status values, counts, and error summaries where applicable.
- Link canonical candle/trade persistence to a data batch when lineage-aware ingestion is used, without polluting canonical market data records with raw payload bodies or GORM tags.
- Reject invalid lineage writes consistently when stable identities are missing or required parent lineage records are missing or unknown.
- Expose batch-scoped audit/replay reads only; direct ingestion-run audit lookup is not part of this change.
- Keep constructors and services small, with consumer-defined interfaces and concrete return types following existing Go conventions.
- Keep SQLite-first tests and PostgreSQL-compatible schema choices consistent with the current data store.

**Non-Goals:**

- No production ingestion scheduler, job orchestration, retry engine, or backfill workflow.
- No backend HTTP routes, UI workflow, or operator controls for lineage in this change.
- No venue adapter rewrite, live network behavior, symbol-mapping framework, or vendor-specific normalization rules.
- No external blob/object storage, compression policy, retention policy, or payload redaction framework beyond avoiding secret-bearing metadata.
- No changes to analytics, strategy, governor, execution, or AI agent behavior.

## Decisions

1. Keep lineage as a `runtime/data` concern first.

   `IngestionRun`, `RawVenuePayload`, `NormalizationRun`, and `DataBatch` should start as data-layer records because they describe ingestion/audit mechanics rather than stable cross-slice market records. This keeps raw payload concepts out of `runtime/domain` unless future consumers prove a shared-kernel need.

   Alternative considered: add all lineage records to `runtime/domain`. That would make them broadly reusable, but it would expose raw payload/audit concepts to deterministic slices that should normally consume canonical market records instead.

2. Persist raw payloads in the existing data store.

   Add GORM models and `AutoMigrate` coverage for lineage tables beside the current data-layer tables. Store payload bytes or text, content type, source, venue, request key, source record identity when available, checksum, and received time. Store only non-sensitive request/response metadata; credentials, auth headers, API keys, cookies, and signatures must not be persisted.

   Alternative considered: use external blob storage immediately. That may become useful for large payload retention, but it adds deployment complexity before the minimal lineage contract is proven.

3. Model normalization and batches separately.

   `NormalizationRun` should capture the attempt to transform raw payloads into canonical records, including status, timestamps, record kind, counts, and error summary. `DataBatch` should group the canonical rows written from a normalization run for a venue/instrument/time range and expose a stable batch identity for audit and replay.

   Alternative considered: collapse everything into one ingestion-run row. That is smaller, but it cannot distinguish raw capture, normalization success/failure, and persisted batch outcomes cleanly.

4. Link canonical rows to batches without changing canonical records.

   Add optional batch linkage in candle/trade persistence models and lineage-aware ingestion/store methods. Returned canonical `domain.Candle` and `domain.Trade` values should remain persistence-free; lineage read methods can return replay rows or audit DTOs carrying batch identities separately.

   Alternative considered: add batch fields to `domain.Candle` and `domain.Trade`. That would make lineage visible everywhere, but it would couple downstream deterministic consumers to ingestion audit details.

5. Keep lineage writes idempotent by external/stable keys.

   Use caller-supplied run IDs, raw payload IDs, normalization IDs, and batch IDs or stable natural keys so ingestion jobs can retry safely without duplicating lineage rows. Repeated writes for the same lineage identity should update mutable status/count/error fields rather than append duplicates.

   Alternative considered: always append a new row per attempt. That preserves every retry as an event log, but it expands the scope into event-sourcing and makes minimal replay/audit lookups harder.

6. Make validation, parent links, and audit ordering explicit.

   Each lineage record should validate its own stable identity and required fields before persistence. Child lineage records must reference existing parent lineage rows: raw payloads require a known ingestion run, normalization runs require one or more known raw payloads, and data batches require a known normalization run. Failed validation or missing parent links must reject the write without persisting partial child lineage.

   Audit reads are batch-scoped in this change. A batch audit result should return the batch, its normalization run, and raw payload audit entries, where each raw payload entry carries its parent ingestion run metadata. When a normalization run has multiple raw payloads, audit entries must be sorted by raw payload received time in UTC ascending, then by stable raw payload identity ascending as the tie-breaker; insertion order is not a contract.

   Alternative considered: add direct run-level audit lookup now. That may be useful later, but it broadens the read contract before the minimal batch replay path is proven.

## Risks / Trade-offs

- Raw payload size can grow database files -> Keep v0 simple with database storage, document that retention/blob offload is out of scope, and avoid introducing scheduler or retention policy now.
- Raw payloads may contain sensitive metadata -> Persist payload body and allowlisted non-sensitive metadata only; never persist credentials, auth headers, cookies, signatures, or API keys.
- Optional batch links may leave old rows without lineage -> Treat lineage linkage as present only for lineage-aware ingestion; existing canonical rows remain valid but audit lookup can report no batch lineage.
- Batch-only audit reads may not satisfy future operator workflows -> Keep this slice focused on persisted batch replay; add direct run-level audit in a later OpenSpec change if needed.
- Idempotency keys may be chosen poorly by callers -> Validate required IDs, trim values, and test duplicate writes; leave higher-level key design to ingestion jobs.
- Schema grows in an early slice -> Keep models explicit, isolated from canonical domain records, and covered by SQLite-backed migration tests.

## Migration Plan

1. Add data-layer lineage records and validation tests without changing existing ingestion behavior.
2. Add GORM lineage tables plus optional `data_batch_id` links on candle/trade rows; include them in `DatabaseStore.AutoMigrate`.
3. Add lineage-aware persistence methods and batch-scoped read/audit methods while preserving existing candle/trade APIs.
4. Add lineage-aware batch ingestion tests that prove canonical rows can be traced back to a batch, normalization run, raw payload, and ingestion run.
5. Rollback is removal of unused lineage write/read paths and tables while existing canonical data tables continue to work; because the project is early, destructive cleanup can be handled by a later explicit migration if needed.

## Open Questions

- Should future payload retention move large raw bodies to object storage while keeping checksums and metadata in SQL?
- Which future ingestion job should be the first real caller of the lineage-aware API?
