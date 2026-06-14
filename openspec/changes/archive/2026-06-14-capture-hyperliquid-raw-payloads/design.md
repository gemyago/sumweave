## Context

`runtime/venueedge` currently calls Hyperliquid `/info`, decodes the response, and returns canonical records. `runtime/data` already has ingestion-run, raw-payload, normalization-run, and batch lineage, but raw payload bodies are modeled as database bytes and the Hyperliquid adapter does not populate lineage before normalization.

This change bridges those pieces for public Hyperliquid market-data reads only. Venue-specific HTTP details stay at the venue edge, and the data layer owns durable raw evidence, blob references, normalized-record links, and replay/audit persistence.

## Goals / Non-Goals

**Goals:**

- Capture Hyperliquid `/info` raw request/response evidence for `meta`, `candleSnapshot`, and v0 deterministic `recentTrades` reads before decoding into canonical records.
- Store raw response bodies through a local file/blob abstraction for v0 and persist only body references plus hashes and compact metadata in SQL.
- Keep canonical `domain.Instrument`, `domain.Candle`, and `domain.Trade` records free of raw payload bodies, GORM tags, and persistence-only lineage fields.
- Link captured raw payloads to normalized records persisted from them, including instruments from `meta` and candles/trades from market-data reads.
- Preserve existing idempotent normalized-record persistence while appending a fresh raw payload evidence record for every new HTTP fetch.
- Keep tests mocked/offline for Hyperliquid HTTP behavior.

**Non-Goals:**

- No live Hyperliquid network requirement in default tests.
- No external object storage, retention policy, compression policy, or blob garbage collection.
- No private Hyperliquid endpoints, signing, orders, fills, wallet actions, or execution behavior.
- No backend API or UI surface for browsing captured payloads.
- No generic venue capture framework beyond what is needed to keep the Hyperliquid implementation testable and the boundary clean.

## Decisions

1. Store raw body bytes behind a local blob interface.

   Add a `runtime/data` blob abstraction such as a local filesystem-backed raw payload body store. The blob store writes immutable body bytes and returns a stable reference plus hash metadata. SQL stores `payload_body_ref` and response hashes, not body bytes.

   V0 ownership and wiring: `runtime/data` owns the `RawPayloadBlobStore` interface and local filesystem implementation; the application/wiring layer owns selecting the base path. In `apps/signal-foundry`, add config such as `dataLayer.rawPayloadBlobStore.path`, defaulting to a path under the configured `dataDir` when unset (for example `data/raw-payloads`). The app constructs `data.NewLocalRawPayloadBlobStore(basePath)` and injects it into `data.NewLineageService` through `LineageServiceDeps`. Keep `data.NewDatabaseStore` and `DatabaseStoreOpts` database-only; the database store persists raw-payload metadata and `payload_body_ref`, while the lineage service coordinates body writes plus SQL metadata persistence.

   Alternative considered: continue storing raw bodies in the `raw_venue_payloads` table. That is simpler but conflicts with the v0 requirement to keep DB rows as references and makes later blob/offload migration harder.

2. Treat each HTTP exchange as a distinct raw evidence record.

   Hyperliquid capture IDs should be unique per completed HTTP exchange. `request_payload_hash` supports grouping repeated logical requests, but repeated fetches append new raw evidence rows and do not overwrite earlier captures. Exact retry of the same generated evidence ID can remain idempotent at the persistence layer if needed for crash recovery.

   Alternative considered: upsert by request hash. That would lose evidence for repeated public reads whose responses differ or whose timing/status matters for debugging.

3. Keep venue-edge results canonical, with lineage references in read-result metadata.

   The adapter should still return canonical records through the existing `MarketDataVenue` methods. Choose read-result metadata for the v0 handoff: add a minimal raw capture metadata field to `InstrumentReadResult`, `CandleReadResult`, and `TradeReadResult` (for example `RawPayloadIDs []string`, empty when capture is disabled). Do not add raw payload fields to `domain.Instrument`, `domain.Candle`, or `domain.Trade`, and do not change the `MarketDataVenue` method signatures.

   `IngestionFlow` reads this metadata after a successful venue read and, after canonical persistence succeeds, records raw-to-normalized links through an optional lineage/link sink. Existing venues that return empty metadata and existing ingestion callers without the optional linkage sink continue unchanged. Hyperliquid error responses are still captured by the adapter before returning an error, but no normalized-record links are written because no canonical records were accepted.

   Alternative considered: add an optional ingestion-only read interface that returns raw payload identities separately from the standard results. That would preserve result structs exactly, but it would duplicate each read method or add adapter-specific branching in ingestion. Read-result metadata is the smaller additive boundary because callers can ignore it and canonical records remain unchanged. Also considered adding raw payload IDs directly to `domain.Instrument`, `domain.Candle`, or `domain.Trade`; that would pollute the shared canonical domain with audit persistence concerns.

4. Make ingestion-run context optional for direct adapter reads.

   The raw payload record should store an ingestion run ID when a lineage-aware ingestion flow provides one. Direct mocked adapter reads can still capture standalone evidence without a run ID; if a non-empty run ID is provided, the data layer must validate that the parent run exists. The v0 recorder dependency passed to `HyperliquidPerpsVenueParams` should accept an optional ingestion run ID in its capture params. Ingestion-aware construction/wiring supplies a recorder scoped with the current run ID; direct adapter construction either omits the recorder or uses a recorder with an empty run ID.

   Alternative considered: require every capture to have an ingestion run. That would force direct adapter reads to invent runs and would broaden orchestration before it exists.

5. Link raw payloads to normalized records using data-layer audit tables.

   Add link persistence that associates a raw payload ID with canonical normalized record identity/kind after successful persistence. Instruments can link directly by venue/symbol; candles and trades can link by persisted canonical replay identity or existing data-batch lineage where appropriate. The link table belongs in `runtime/data`, not `runtime/domain`.

   Alternative considered: rely only on data batches. Existing data-batch lineage covers candles/trades well, but `meta` instrument reads also need raw-to-normalized evidence linkage.

6. Capture error responses before returning errors.

   The Hyperliquid HTTP helper should record request hash, timestamps, status, body hash, and body reference before decoding or converting non-2xx responses to errors. This makes debugging failed or malformed venue responses possible without another venue call.

   Alternative considered: capture only successful responses. That would miss the most useful debugging evidence.

7. Keep the Hyperliquid capture dependency narrow and data-agnostic.

   Add a small consumer-defined capture interface in `runtime/venueedge` near `HyperliquidPerpsVenue`, for example a `hyperliquidRawEvidenceRecorder` accepted by `HyperliquidPerpsVenueParams`. Its params should contain only the already-available HTTP exchange evidence and optional run/scope metadata; it should return the raw payload ID/reference metadata needed for read-result handoff. The adapter should not import GORM, file storage, or concrete data-layer persistence types.

## Risks / Trade-offs

- Blob files may accumulate locally -> Keep v0 scope to local durable writes and leave retention/GC for a later change.
- Captured response bodies may contain unexpected sensitive data -> Only public `/info` endpoints are in scope; request metadata remains allowlisted and secret-bearing headers/values are not persisted.
- Optional lineage metadata can complicate result types -> Keep raw references out of canonical domain records and isolate them to one optional raw-capture metadata field on read results.
- Recent trades are not backfillable through `/info` -> Capture only the current deterministic single response window that the adapter already supports; do not add paging or historical backfill.
- Schema migration changes existing raw payload storage -> Because the project is early, update the GORM schema and tests directly; rollback is restoring body-in-DB fields and dropping new reference/link columns in a later explicit cleanup if needed.

## Migration Plan

1. Add/modify data-layer lineage records and SQLite migration tests for body references, request/response evidence metadata, optional run ID, and raw-to-normalized links.
2. Implement the local raw payload blob store and database persistence that stores only references/hashes in SQL; wire `LineageServiceDeps` to require or accept the blob store for raw body writes.
3. Add app/runtime wiring for the v0 local blob-store base path (`dataLayer.rawPayloadBlobStore.path`, default under `dataDir`) and inject the blob store into the data lineage service.
4. Add Hyperliquid mocked-HTTP capture tests, then instrument `/info` request execution so evidence is written before response decoding and raw payload IDs are returned via read-result metadata.
5. Update lineage-aware ingestion flow to record raw-to-normalized links from read-result metadata after canonical persistence while preserving existing non-lineage ingestion behavior.
6. Existing local development databases can be recreated during early development; production migration hardening is out of scope until deployment needs require it.

## Open Questions

- None for planning. V0 chooses read-result metadata for raw payload ID handoff and app/runtime-owned local blob-store path injection.
