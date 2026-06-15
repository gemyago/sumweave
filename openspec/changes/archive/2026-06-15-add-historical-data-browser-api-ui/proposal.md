## Why

Operators can now populate historical Hyperliquid raw candle payloads and canonical candle rows through the merged backfill CLI, but there is no read-only way to inspect that durable data from the operator surface. The next useful product slice is a protected historical data browser that makes normalized candle rows, raw payload metadata, bounded raw body previews, and raw-to-candle lineage evidence visible without adding ingestion controls or trading actions.

## What Changes

- Add app-owned protected `GET /api/v1/data/*` routes for historical candle browsing, raw payload metadata browsing, bounded raw payload detail previews, and candle-linked raw evidence lookup.
- Make candle browsing deterministic by rejecting requests above 10,000 requested candle intervals with `400 Bad Request`.
- Make candle-linked evidence lookup unambiguous by requiring the selected candle's `provenanceSource` and `provenanceIdentity` query fields.
- Reuse `runtime/data` deterministic candle replay/query reads and add explicit raw payload browser read contracts/result DTOs that keep GORM models and raw body storage isolated from HTTP DTOs.
- Add a protected `#/data` UI route with filters, summary, normalized-candle candlestick chart, raw payload metadata table, candle evidence panel, and bounded raw payload detail drawer.
- Add `Data` to the authenticated nav and update `apps/signal-ui/ui-wireframe.md` for route, layout, states, and behavior.
- Keep the slice read-only: no backfill start, scheduler, mutation, repair, re-normalization, trading, analytics overlays, or unbounded body download.

## Capabilities

### New Capabilities

- `historical-data-browser`: Protected operator workflow for browsing historical normalized candles and raw Hyperliquid payload lineage evidence.

### Modified Capabilities

- `data-layer`: Raw payload and raw-to-normalized lineage read models become queryable for browser/API use.

## Impact

- Affects `runtime/data` lineage/read contracts, `DatabaseStore`, `LineageService`, blob preview reading, and focused data-layer tests.
- Affects `apps/signal-foundry/internal/api/http/v1routes.yaml`, generated `v1routes/`, new `v1controllers` data controller, protected route registration, and backend tests.
- Affects `apps/signal-foundry/internal/runtime.go` only for dependency exposure/wiring if existing `Runtime` fields are insufficient.
- Affects `apps/signal-ui/src/App.svelte`, `src/components/Nav.svelte`, new `src/pages/Data.svelte`/data components, a data API client wrapper, tests, `package.json`, and `package-lock.json` for `lightweight-charts`.
- Affects `apps/signal-ui/ui-wireframe.md`; no AGENTS update is expected unless implementation changes commands or workflow guidance.
