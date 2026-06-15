## Why

Operators need a first manual path to ingest historical Hyperliquid candle data with durable raw evidence and lineage before adding schedulers, backend routes, or UI workflows. The platform already has data-layer lineage, Hyperliquid raw capture, canonical candle ingestion, and app wiring, so the next product slice is a CLI-first backfill runner that composes those foundations safely.

## What Changes

- Add a manual `signal-foundry data backfill-raw-candles` CLI command for Hyperliquid perps candles only.
- Accept explicit venue, symbol, asset class, timeframe, half-open start/end range, run ID, and optional page size inputs with deterministic validation errors.
- Add a reusable runtime backfill runner outside Cobra that records ingestion-run lifecycle, invokes Hyperliquid candle reads with `RawEvidenceIngestionRun`, persists canonical candles through the existing venue ingestion flow, and links raw payload evidence to persisted candle rows.
- Produce a deterministic human-readable completeness/gap report covering expected count, persisted count, first/last persisted boundaries, missing intervals, duplicate natural keys when detected, and raw payload count when available cheaply, otherwise omitting that field deterministically.
- Keep historical trades explicitly out of scope and do not add scheduler, backend HTTP routes, UI screens, private Hyperliquid behavior, external object storage, or AI calls.

## Capabilities

### New Capabilities

- `historical-data-backfill`: Manual operator-triggered historical Hyperliquid raw candle backfill via runtime runner and CLI command.

### Modified Capabilities

- None.

## Impact

- Affects `runtime/flows` or an equivalent thin orchestration package for the reusable historical raw candle backfill runner and gap-report logic.
- Affects `runtime/data` only through existing ingestion, lineage, and read/replay service interfaces; no new persistence model is expected unless duplicate/raw-payload-count readback proves unavailable and is explicitly scoped during implementation.
- Affects `runtime/venueedge` through existing Hyperliquid candle read, raw evidence metadata, and ingestion-flow linkage behavior; no generic multi-venue framework should be introduced.
- Affects `apps/signal-foundry/cmd/signal-foundry` and `apps/signal-foundry/internal` for Cobra command wiring, dependency resolution, runtime config reuse, CLI output, and command tests.
- Verification should use focused unit tests and mocked-HTTP integration-style tests; default validation must not require live Hyperliquid network access.
