## Why

The current historical data browser is technically useful but starts from a blank filter form that requires an operator to already know venue, symbol, asset class, timeframe, start, and end. Raw payload browsing is more flexible, but optional raw filters do not solve discovery for normalized candles. The chosen product direction is browse-first discovery based on persisted normalized candle availability: operators should see what venue + symbol + asset class entries already have canonical candle data and be able to start browsing by selecting one entry.

## What Changes

- Add a read-only normalized candle availability / browse seed read path based on persisted canonical candle rows.
- Expose an authenticated app-owned data endpoint with a fixed contract: optional exact `venue`, `symbol`, and `assetClass` filters; deterministic top-level entry pagination with `limit` and opaque `cursor`; and one availability item per venue + symbol + asset class.
- Make each availability item include non-empty timeframe summaries, deterministic per-entry default slice fields, and counts/ranges derived only from persisted canonical candle rows; make the first page include a top-level default selection when data exists.
- Update `#/data` so it loads availability first, renders an easy list of available normalized candle entries, selects the response default when available, auto-loads only the bounded default candle slice, and selects a default candle when the slice has data.
- Keep the existing candle query endpoint as the exact replay/browse path that requires venue, symbol, asset class, timeframe, start, and end; the availability endpoint supplies those defaults instead of making operators guess them.
- Keep raw payload browsing and candle-linked evidence secondary to the selected normalized candle scope; do not use raw-only payloads or live venue symbol lists as discovery seeds, and do not auto-load the broad raw payload metadata list from the initial availability load.
- Make the authenticated default route and post-login fallback land on data rather than chat when no explicit route was requested, while preserving explicit chat access and deep-link return.
- Keep the whole slice read-only: no backfill starts, scheduled ingestion, gap filling, repair, re-normalization, or data mutation.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `historical-data-browser`: Becomes browse-first for normalized candle discovery and default-entry behavior.
- `data-layer`: Adds read-only normalized candle availability read models derived from persisted canonical candles.

## Impact

- Affects `runtime/data` read contracts and store queries for grouped normalized candle availability and deterministic default slice derivation.
- Affects `apps/signal-foundry/internal/api/http/v1routes.yaml`, generated route code, and data controller tests for the new authenticated availability endpoint.
- Affects `apps/signal-ui` data client, protected routing/default route behavior, `#/data` page state, availability list UI, default auto-load behavior, candle selection behavior, and focused tests.
- Affects `apps/signal-ui/ui-wireframe.md` if implementation updates documented route/layout behavior.
- No new ingestion workflow, scheduler, mutation endpoint, live venue discovery, or generic data platform redesign is expected.
