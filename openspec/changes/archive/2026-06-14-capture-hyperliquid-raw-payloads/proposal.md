## Why

Hyperliquid public market-data reads are normalized immediately, so later replay and debugging still depend on calling `/info` again. Capturing the raw request/response evidence before normalization gives future replays a durable source artifact while keeping downstream records canonical.

## What Changes

- Capture raw Hyperliquid `/info` request/response evidence for `meta`, `candleSnapshot`, and deterministic single-window `recentTrades` reads before JSON normalization.
- Extend data-layer raw payload lineage so evidence stores request payload hash, compact non-secret request metadata, request/response timestamps, HTTP status, raw response body hash, raw response body reference, entity hint, optional instrument/timeframe/time range, and optional ingestion run identity.
- Add a v0 local file/blob abstraction for raw response body bytes; database rows store only the body reference and hashes, not the raw body bytes.
- Pass raw payload identities to ingestion through optional read-result metadata on the existing venue-edge result structs; canonical domain records and `MarketDataVenue` method signatures stay unchanged.
- Own v0 blob-store path selection in app/runtime wiring, construct the local blob store from config, and inject it into the data lineage service while keeping `DatabaseStore` focused on SQL metadata.
- Link raw payload evidence to the normalized instrument, candle, or trade records produced from it without adding raw payload fields to canonical domain records.
- Ensure each repeated Hyperliquid fetch creates a new raw payload evidence row even when the request hash and normalized record natural keys are the same.
- Preserve the existing venue-edge contract that returns canonical `domain.Instrument`, `domain.Candle`, and `domain.Trade` records.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `data-layer`: Store raw payload evidence through body references, support optional ingestion-run context, and link raw payloads to normalized records.
- `venue-edge`: Capture Hyperliquid `/info` raw evidence for supported public market-data request types while still emitting canonical records.

## Impact

- Affects `runtime/data` lineage domain, GORM models/migrations, lineage store/service methods, local blob storage, and tests.
- Affects `runtime/venueedge` Hyperliquid adapter request execution, optional capture dependencies, read-result metadata, ingestion flow linkage, and mocked-HTTP tests.
- Affects `apps/signal-foundry/internal` data-layer configuration/wiring for the local raw payload blob-store base path.
- Does not add live-network tests, backend API routes, UI workflows, external object storage, private Hyperliquid actions, wallet signing, or execution-slice behavior.
- Completion will require the runtime coding-task protocol, including `make affected-lint-test` from the repository root.
