## Context

The current product direction is `Data -> Analytics -> Strategy -> Governor -> Execution`, with `runtime/` as the core product runtime, `apps/signal-foundry/` as the Go API/jobs app, and `apps/signal-ui/` as the operator UI. The merged historical raw candle backfill CLI persists Hyperliquid raw payload evidence, canonical `domain.Candle` rows, and raw-to-candle links. Existing app wiring already exposes data store/read/lineage services in `Runtime`, and the app-owned OpenAPI surface lives in `apps/signal-foundry/internal/api/http/v1routes.yaml` with apigen-generated route code.

The requested slice is read-only operator browsing. It should use UI copy "normalized candles" while mapping to persisted canonical `domain.Candle` rows. A richer normalization-run/data-batch UX is intentionally deferred.

## Goals / Non-Goals

**Goals:**

- Provide authenticated app-owned HTTP endpoints under `/api/v1/data/*` for candle replay, raw payload metadata list, raw payload detail preview, and candle-linked raw payload evidence.
- Enforce an exact server-side candle browse cap of 10,000 requested candle intervals and return `400 Bad Request` before reading data when a request exceeds it.
- Keep backend DTOs separate from GORM models and `runtime/domain` persistence details.
- Reuse deterministic candle reads (`ReadService.ReplayCandles` when stable identity is useful) and add raw payload browser read models in `runtime/data`.
- Return raw payload body content only from detail, bounded by a deterministic preview limit with a truncation flag and byte size.
- Add a protected `#/data` UI with explicit filters, no auto-query of large ranges, chart-ready candle visualization, tables, drawer, and accessible loading/empty/error states.

**Non-Goals:**

- No UI/API path to start backfills, schedule recurring ingestion, fill gaps, repair, delete, edit, re-normalize, or mutate lineage/market data.
- No historical trades, multi-venue routing framework, live trading controls, backtest/paper/execution controls, strategy/analytics overlays, or decision markers.
- No unbounded raw body download or raw response body in list responses.
- No formal normalization-run/data-batch audit UX in this v0 browser.

## Decisions

1. Keep the new HTTP routes app-owned, not part of the runtime agent API.

   The ticket explicitly asks for `/api/v1/data/*` on the backend app. `runtime/httpapi` remains the agent/profile/session API, while `apps/signal-foundry/internal/api/http` owns operator app routes and auth wrapping.

2. Add runtime data read contracts rather than exposing store/GORM details to handlers.

   `runtime/data` should define query params and DTOs for raw payload metadata, detail preview, and linked payload lookup. `DatabaseStore` can implement the filtering/pagination queries, and `LineageService` can coordinate validation plus blob reads/truncation. Handlers consume these services/interfaces and map to OpenAPI DTOs.

3. Use full natural keys, including provenance, for candle-linked raw payload lookup.

   The UI may receive a replay identity for display, but `/api/v1/data/candle-raw-payloads` must not rely on database row IDs and must not guess across candles that share the same venue/symbol/asset class/timeframe/start/end. The candle endpoint must return `provenanceSource` and `provenanceIdentity` for each normalized candle row, where `provenanceIdentity` is the persisted candle natural-key identity (`provenance_identity_key`, currently derived from the candle provenance record ID). The linked-evidence endpoint must require venue, symbol, asset class, timeframe, candle start/end, `provenanceSource`, and `provenanceIdentity`. Missing or blank provenance fields are request validation errors and return `400 Bad Request`; a fully specified key that has no linked raw payloads returns an empty `items` collection.

4. Enforce the candle range cap by interval count, not by row count after query.

   Define `maxCandleIntervals = 10_000` in the backend data controller/service boundary. Use the supported timeframe duration map (`1m`, `5m`, `15m`, `1h`, `4h`, `1d`) and reject a candle request when `end - start > maxCandleIntervals * duration(timeframe)`. Exactly 10,000 intervals is allowed. This yields deterministic maximum ranges: `1m` = 6d 22h 40m, `5m` = 34d 17h 20m, `15m` = 104d 4h, `1h` = 416d 16h, `4h` = 1666d 16h, and `1d` = 10,000d. The server must return `400 Bad Request` for oversized ranges before calling `ReadService.ReplayCandles`; the UI may mirror this validation, but the server is the source of truth.

5. Use stable, bounded pagination for raw payload metadata.

   The list endpoint should default to 50 and cap at 200, sorted by `receivedAt ASC, id ASC`. Cursor can be an opaque stable pair derived from that order. The list response must never include response bodies.

6. Keep UI client integration consistent with current patterns.

   The UI currently has a handwritten auth client for app-owned auth routes and generated types for the runtime agent API. For this slice, a small typed `src/lib/data/data-api.ts` wrapper is acceptable and likely cheaper than expanding UI OpenAPI generation, as long as tests lock query serialization and response mapping.

7. Make table selection the reliable v0 evidence path.

   `lightweight-charts` should render candlesticks from candle starts and OHLC values. If precise chart hit-testing is awkward, the candle table should be the reliable way to select candles and load linked evidence; chart selection can remain visual-only without blocking acceptance. The selected candle row must carry the `provenanceSource` and `provenanceIdentity` values returned by the candle API and pass them through to linked-evidence requests.

## Risks / Trade-offs

- Large ranges can produce heavy responses: the candle endpoint has an exact 10,000-interval server cap, the UI requires explicit Load, and the route never auto-queries on first render.
- Raw bodies can be large: metadata list excludes bodies; detail returns deterministic preview bytes and truncation metadata only.
- Older/dev data may lack raw-to-candle links: UI should show an empty evidence state, not an error.
- App OpenAPI codegen and UI client generation are currently separate: keep the data client wrapper focused unless implementation deliberately introduces a broader app API generation workflow.
- This is a large full-stack slice: if implementation pressure is high, deliver in chunks while preserving the OpenSpec contract.

## Migration Plan

1. Extend `runtime/data` with raw payload browse query/result types, filtering/pagination, detail preview, and linked raw payload metadata reads, with TDD coverage.
2. Add app OpenAPI data schemas/routes, regenerate `v1routes`, add a protected data controller, and wire dependencies through existing app runtime/DI.
3. Add the UI data API wrapper and `#/data` route/nav entry with filters, chart/table/drawer components, accessibility states, and focused tests.
4. Update `ui-wireframe.md` alongside UI route/layout/state changes and add `lightweight-charts` lockfile updates.

## Deferred Follow-up

- Ingestion-run list/detail is out of scope for this v0 browser; the raw payload list keeps ingestion run ID as an optional free-text filter only.
