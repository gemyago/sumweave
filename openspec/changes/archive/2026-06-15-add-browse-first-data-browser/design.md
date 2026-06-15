## Context

The current `#/data` browser was added as a protected read-only historical data surface. It requires explicit candle filters before loading: venue, symbol, asset class, timeframe, start, and end. That shape is safe, but it forces operators to know what normalized candle data exists before they can browse it.

We considered making filters optional, but the better product direction is discovery from normalized candle availability. The user explicitly chose option B: discovery should be based on venue + symbol entries that already have browsable normalized candle data, not merely raw payloads and not known live venue symbols. Raw payload browsing remains useful as evidence for selected candles, but it is not the primary entry point.

## Goals / Non-Goals

**Goals:**

- Let an authenticated operator open `#/data` and immediately see available normalized candle entries without manually filling every candle query field.
- Base discovery only on persisted canonical candle rows and their normalized availability.
- Provide deterministic default selection and a bounded default candle slice so selecting one available entry is enough to begin browsing.
- Select a candle by default when the auto-loaded/default slice contains at least one candle.
- Preserve the existing exact candle browsing API contract for explicit venue/symbol/assetClass/timeframe/range reads.
- Keep the data browser read-only and safe.
- Make data the authenticated default route instead of chat, without removing explicit chat navigation.

**Non-Goals:**

- No generic data catalog, data marketplace, or multi-source platform redesign.
- No discovery seeded from raw payloads that did not produce normalized candles.
- No discovery seeded from live venue symbol/reference APIs unless those symbols also have persisted candles.
- No historical trade browser, normalization-run/data-batch audit UX, analytics overlays, strategy/backtest controls, or trading actions.
- No ingestion, backfill, scheduler, repair, gap-fill, delete, edit, or re-normalization action.

## Decisions

1. Use persisted normalized candles as the discovery source.

   Availability is derived from canonical candle rows only. A venue + symbol that exists in raw payload metadata but has no persisted candle rows must not appear as an available browse entry. A live venue symbol that has not been persisted as candles must not appear either.

2. Add a dedicated availability/read seed endpoint rather than weakening the candle endpoint.

   `GET /api/v1/data/candles` should continue to require exact venue, symbol, asset class, timeframe, start, and end. A separate availability endpoint can answer "what can I browse?" and provide defaults that the UI passes to the existing candle endpoint.

3. Derive default slices deterministically and keep them bounded.

   The availability read model returns top-level availability items, one per `venue` + `symbol` + `assetClass`. Each item contains all persisted timeframe summaries for that entry and a per-entry `defaultSlice`. The default slice is based on the item’s most recently available timeframe group, using timeframe duration ascending as the stable tie-breaker. The top-level response includes `defaultSelection` only on the first page and only when at least one item exists; it mirrors the first returned item’s deterministic default slice. Every default range ends at the selected timeframe group’s latest available candle end and starts at the later of that group’s earliest available candle start and `latestEnd - 500 * duration(timeframe)`. Missing intervals inside the slice are not filled.

4. Fix the availability request and pagination contract.

   `GET /api/v1/data/candle-availability` accepts optional exact `venue`, `symbol`, and `assetClass` filters, plus `limit` and opaque `cursor`. It does not accept timeframe, start, end, raw-payload, ingestion-run, or live venue discovery filters in this change. Pagination is over top-level venue + symbol + asset class items after filters are applied. The default limit is 50, the maximum limit is 200, and invalid filters, limits, or cursors produce deterministic 4xx validation errors.

5. Make browse-first UI behavior automatic but not mutating.

   On `#/data`, the UI calls the availability endpoint and then, when the first page contains a `defaultSelection`, calls the candle endpoint for that exact default slice. These are read-only calls and replace the current blank-first state when availability exists. If no availability exists, the UI shows an empty state and must not call the candle endpoint with guessed filters.

6. Treat raw payloads as evidence after a normalized candle scope exists.

   The initial availability/default-candle path does not auto-call the broad `GET /api/v1/data/raw-payloads` metadata list. When a candle is default-selected or manually selected, the UI calls the candle-linked evidence endpoint using that candle’s provenance. The raw payload metadata list remains available through explicit Load/filter behavior scoped to the selected normalized candle entry/range, but it must not determine which symbols are shown as browseable normalized candle entries.

7. Prefer data as the authenticated landing surface.

   The current product flow is data-first. Authenticated app opens with no explicit route, empty hash, or login completion without a saved destination land on `#/data`. If the user requested an explicit protected route such as `#/chat` or `#/data` before login, post-login navigation returns to that explicit route. Explicit `#/chat` remains available through navigation or direct URL.

## Risks / Trade-offs

- Availability queries can become expensive if implemented by scanning candles naively; implementation should use grouped reads and indexes where needed, but should not introduce writes just to cache availability in this slice.
- Auto-loading a default candle slice changes first-render behavior; the default range is bounded to avoid large accidental reads.
- Operators may still need explicit range controls for older data; the existing filter form remains available as an edit path after selecting an entry.
- Existing raw payload optional filters remain useful, but they are intentionally not promoted to primary discovery because they can include data that has not produced normalized candles.

## Migration Plan

1. Add `runtime/data` read contracts and store coverage for normalized candle availability, grouped ranges/counts, deterministic ordering, and default slice derivation.
2. Add the authenticated app data availability endpoint and controller mapping without changing the existing candle replay endpoint contract.
3. Update the UI data client and `#/data` page to load availability first, render paginated entries with timeframe summaries and per-entry defaults, auto-load the first-page default candle slice, default-select a returned candle, and load linked evidence for selected candles without broad raw metadata auto-load.
4. Update authenticated default and post-login fallback routing to data while preserving explicit route/deep-link navigation, including chat.

## Deferred Follow-up

- Rich normalization-run/data-batch browsing and broader data catalog concepts remain separate follow-ups.
- Persisted availability cache tables should only be considered later if grouped read performance requires them.
