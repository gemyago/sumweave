## 1. Data Lineage Domain And Service Boundary (`lineage-contracts`)

- [x] 1.1 Add `runtime/data` lineage records and validation for `IngestionRun`, `RawVenuePayload`, `NormalizationRun`, and `DataBatch`, including rejection of missing stable identities and missing/unknown required parent links; follow TDD by writing failing data-layer tests first, then implementing constructors/canonicalization, then verifying focused tests.
- [x] 1.2 Add minimal lineage service/store contracts for creating or updating runs, raw payloads, normalization runs, batches, and batch-scoped audit/replay reads without changing existing candle/trade ingestion behavior or adding direct run-level audit lookup; follow TDD by writing failing service tests with local fakes first, then implementing, then verifying focused tests.

## 2. GORM Lineage Persistence (`gorm-lineage-persistence`)

- [x] 2.1 Add GORM lineage models, explicit table/column names, indexes, UTC timestamp handling, and `AutoMigrate` coverage for ingestion runs, raw venue payloads, normalization runs, and data batches; follow TDD by writing failing SQLite migration/schema tests first, then implementing, then verifying focused tests.
- [x] 2.2 Implement idempotent database store methods for lineage upserts and batch-scoped audit reads while avoiding persistence of secret-bearing metadata; follow TDD by writing failing SQLite store tests for duplicate writes, status/count updates, missing/unknown parent rejection, payload checksum/body persistence, stable raw-payload audit ordering, and secret metadata exclusion first, then implementing, then verifying focused tests.

## 3. Batch-Linked Canonical Persistence And Audit (`batch-audit-replay`)

- [x] 3.1 Add lineage-aware candle/trade batch ingestion or store methods that associate canonical rows with a `DataBatch` while preserving existing non-lineage ingestion APIs; follow TDD by writing failing service/store tests for batch-linked candle and trade writes first, then implementing, then verifying focused tests.
- [x] 3.2 Add read/audit coverage that traces persisted canonical rows from batch to normalization run, ordered raw payload audit entries, and parent ingestion run metadata with stable replay identities; follow TDD by writing failing SQLite-backed tests for batch lineage lookup and repeated replay first, then implementing, then verifying focused tests.
