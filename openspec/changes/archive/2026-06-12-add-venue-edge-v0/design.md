## Context

Signal Foundry's deterministic path is `Data -> Analytics -> Strategy -> Governor -> Execution`, with venue integration isolated at the system edge. Data Layer v0 already stores canonical instruments, candles, trades, provenance, and replayable reads. The next useful slice is a venue edge that proves how external market-data sources become canonical domain records without making the data layer aware of exchange payloads, signing, pagination, or symbol quirks.

## Goals / Non-Goals

**Goals:**

- Add a small market-data venue edge that can produce canonical instruments, candles, and trades.
- Provide a deterministic sandbox venue for repeatable development and integration tests.
- Prove sandbox-to-data ingestion with automated tests against the existing data-layer contract.
- Add a first real venue adapter tested against documented HTTP shapes using a local HTTP test server.
- Keep abstractions narrow enough that the first real venue can still reshape the edge if sandbox assumptions prove wrong.

**Non-Goals:**

- No live real-venue E2E tests in v0.
- No order placement, fills, reconciliation, balances, positions, or execution behavior.
- No generic multi-venue framework beyond the narrow market-data edge needed now.
- No operator UI, scheduling system, production ingestion jobs, or data repair workflow.
- No AI behavior inside market-data ingestion or the deterministic path.

## Decisions

### Decision: Start with market-data venue edge only

Create a venue edge focused on instruments, candles, trades, source identity, pagination/range concerns, and mapping into `runtime/domain` records. Execution-facing venue behavior remains out of scope until the execution slice exists.

Rationale: the data layer is ready to consume canonical market data now, while order/fill/reconciliation behavior belongs later after governor/execution boundaries are clearer.

Alternative considered: define a full venue abstraction for market data and trading at once. That would front-load too many assumptions and risks turning v0 into a generic exchange framework.

### Decision: Keep sandbox data synthetic but deterministic

The sandbox venue should generate plausible records from stable inputs such as seed, venue, symbol, timeframe, and time range. Generated records should have stable provenance and source record identifiers.

Rationale: tests and demos need data that feels venue-shaped but remains reproducible. Pure randomness would make ingestion behavior harder to debug and would conflict with the deterministic product stance.

Alternative considered: use static fixture files only. Fixtures are useful for scripted edge cases, but they do not exercise range, paging, and repeated ingestion behavior as naturally as a deterministic generator.

### Decision: Let sandbox shape the edge, but verify quickly with a real adapter

Build the sandbox first, then add one real venue adapter soon after with mocked HTTP responses that match the selected venue's documented API shape. Keep the adapter concrete rather than introducing a broad registry or framework.

Rationale: the sandbox gives fast local feedback, but the real adapter keeps the design honest about transport, payload, pagination, timestamp, and symbol-mapping quirks.

Alternative considered: build the real adapter first. That would expose real constraints earlier, but it slows down the first integration loop and makes repeatable data-layer tests more dependent on fixture completeness.

### Decision: Use local HTTP test servers for real venue validation

Real venue adapter tests should use Go's `httptest` package or equivalent local test HTTP server with documented response payloads. Default tests should not call live venue APIs.

Rationale: live venue E2E tests are vulnerable to credentials, rate limits, network state, market availability, and upstream payload drift. Mocked HTTP still validates URL construction, response parsing, paging, error handling, and data-layer ingestion without external flake.

Alternative considered: include opt-in live smoke tests in v0. That may be useful later, but it is not required to prove the venue edge and would expand the current scope.

### Decision: Integrate through the existing data-layer service

Venue integration tests should pass canonical records through `runtime/data.IngestionService` and read them back through the data-layer read/replay APIs. The venue edge should not write directly to data persistence.

Rationale: this proves the intended boundary: venues produce canonical records, while the data layer owns validation, idempotency, persistence, and replay semantics.

Alternative considered: give venue adapters direct store access. That would reduce wiring but would duplicate or bypass the data-layer contract.

## Risks / Trade-offs

- Sandbox assumptions become fantasy architecture -> Add the first real adapter in the same change and allow its mocked-HTTP behavior to reshape the edge.
- Synthetic data hides hard market-data cases -> Include scripted sandbox scenarios or fixtures for gaps, duplicates, spikes, malformed ranges, and paging boundaries when needed.
- Real adapter tests drift from documentation -> Keep documented payload examples close to tests and make the selected venue/API version explicit.
- Venue edge grows into a framework too early -> Add only the interfaces and structs consumed by the current ingestion flow; defer registries, plugin systems, and execution behavior.
- Live venue behavior remains unverified -> Accept this in v0 and track live smoke/E2E as a future, opt-in capability after mocked HTTP integration is stable.

## Migration Plan

1. Add the venue edge package or runtime area without changing existing data-layer behavior.
2. Implement and test sandbox generation before introducing a real adapter.
3. Add sandbox-to-data integration tests using SQLite-backed data-layer storage.
4. Add one selected real venue adapter with mocked HTTP tests and mocked-HTTP-to-data integration coverage.
5. Roll back by leaving the data layer unchanged and removing unused venue edge wiring if the shape proves wrong.

## Open Questions

- First real venue target selected for v0 implementation: Binance Spot public REST market-data endpoints using `GET /api/v3/exchangeInfo`, `GET /api/v3/klines`, and `GET /api/v3/aggTrades` with mocked HTTP coverage only.
- Should candles be generated directly from the sandbox price path, derived from generated trades, or both with a consistency check?
- What is the smallest useful paging model for the sandbox without overfitting to the first real venue?
- Should v0 expose any app-level manual command or remain runtime-test-only until production ingestion jobs are designed?

## Follow-up Notes

- Live real-venue E2E remains out of scope for v0 completion; if needed later, track it as a separate opt-in future change after mocked-HTTP adapter coverage remains stable.
