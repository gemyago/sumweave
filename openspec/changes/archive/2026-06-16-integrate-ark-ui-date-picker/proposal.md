## Why

Operators currently edit UTC start/end values through free-entry text inputs in the UI, including `apps/signal-ui/src/pages/Data.svelte` and `apps/signal-ui/src/pages/Evaluations.svelte`, which is error-prone for deterministic data browsing and backtest workflows. Ark UI provides a fresh Svelte-compatible date/range picker foundation that can preserve UTC-first behavior while making common product ranges easier to select.

## What Changes

- Add an Ark UI based UTC-aware date range picking capability for the Signal Foundry UI.
- Replace free-entry date text boxes on the historical data browser and evaluation runner flows with validated range picker interactions while retaining explicit UTC values.
- Support quick-select range presets appropriate for deterministic workflows, including Last 24h, Last 7d, Last 30d, Last 90d, and Last 180d.
- Ensure selected ranges remain half-open, UTC-first, and compatible with timeframe/range constraints such as the data browser candle interval cap.
- Prefer shared componentization in `apps/signal-ui/` so data browsing and evaluation workflows use consistent range validation and copy.

## Capabilities

### New Capabilities
- `utc-date-range-picker`: Shared Signal Foundry UI capability for Ark UI based UTC date/range selection and deterministic quick-select presets.

### Modified Capabilities
- `historical-data-browser`: Replace manual UTC range entry on the data browser with the shared UTC-aware range picker while preserving exact candle-scope validation and range caps.
- `strategy-workspace`: Replace manual evaluation range entry with the shared UTC-aware range picker for deterministic backtest/evaluation starts.

## Impact

- Affects `apps/signal-ui/`, especially `src/pages/Data.svelte`, `src/pages/Evaluations.svelte`, and likely shared component/support modules for date range selection.
- Adds or updates UI dependencies for Ark UI Svelte date picker primitives rather than flatpickr.
- Does not change backend API contracts, persistence schemas, or runtime backtest/data semantics.
