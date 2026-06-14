## 1. Data Raw Evidence Storage (`data-raw-evidence-storage`)

- [x] 1.1 Add data-layer raw payload evidence fields, validation, and optional ingestion-run behavior, and follow TDD by writing failing lineage tests for required hashes/metadata/timestamps/body refs, standalone capture, known/unknown run references, repeated-fetch append semantics, and same-ID retry idempotency before implementing and verifying focused tests.
- [x] 1.2 Add the v0 local raw payload blob store and database body-reference persistence, and follow TDD by writing failing local blob, lineage-service, and SQLite migration/store tests proving body bytes are written to the blob store, only `payload_body_ref` and hashes are stored in DB, explicit columns exist, `LineageServiceDeps` receives the blob store while `DatabaseStoreOpts` remains database-only, and secret metadata is excluded before implementing and verifying focused tests.
- [x] 1.3 Add raw-payload-to-normalized-record link persistence for instruments, candles, and trades, and follow TDD by writing failing data-layer tests for successful links, unknown raw payload rejection, stable audit/readback, and no raw fields on canonical returned records before implementing and verifying focused tests.
- [x] 1.4 Wire v0 app/runtime blob-store configuration so `apps/signal-foundry` selects `dataLayer.rawPayloadBlobStore.path` (defaulting under `dataDir` when unset), constructs the local blob store, and injects it into the data lineage service, and follow TDD by writing failing config/wiring tests before implementing and verifying focused tests.

## 2. Hyperliquid Raw Capture (`hyperliquid-raw-capture`)

- [x] 2.1 Add optional Hyperliquid raw evidence capture around `/info` HTTP execution, and follow TDD by writing failing mocked-HTTP tests for `meta`, `candleSnapshot`, deterministic `recentTrades`, non-2xx/malformed responses, request/response timestamps, HTTP status, request payload hash, response body hash/ref, entity hints, optional ingestion run ID in recorder params, raw payload IDs on read-result metadata, and canonical record return behavior before implementing and verifying focused tests.
- [x] 2.2 Add repeated-fetch behavior for Hyperliquid capture, and follow TDD by writing failing mocked-HTTP tests proving identical repeated reads create distinct raw payload evidence records while adapter canonical results and downstream normalized idempotency remain unchanged before implementing and verifying focused tests.

## 3. Ingestion Linkage (`ingestion-raw-linkage`)

- [x] 3.1 Wire lineage-aware ingestion so captured Hyperliquid raw payload IDs from read-result metadata link to persisted normalized instruments, candles, and trades, and follow TDD by writing failing ingestion-flow tests with mocked venues/sinks for inside-run IDs, raw payload links, batch or replay identities, empty metadata behavior, and unchanged non-lineage ingestion behavior before implementing and verifying focused tests.
