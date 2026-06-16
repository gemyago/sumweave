## Context

`apps/signal-ui/` is the active Svelte 5 operator UI. The historical data browser (`src/pages/Data.svelte`) and evaluation runner (`src/pages/Evaluations.svelte`) currently expose UTC start/end values as free-entry text fields, even though both flows depend on precise half-open time ranges for deterministic candle replay, evidence lookup, and backtest evaluation.

The UI needs a better interaction without weakening the product’s UTC-first and deterministic behavior. Ark UI is the preferred dependency because its Svelte primitives meet the project’s freshness/compatibility expectations and support date picking, range selection, presets/composability, UTC/timezone control, and later time integration. Flatpickr is not preferred because it does not align as well with the Svelte/component freshness direction for this codebase.

## Goals / Non-Goals

**Goals:**

- Introduce an Ark UI based, Signal Foundry styled UTC range picker for the UI.
- Reuse the same range selection behavior in data browsing and evaluation workflows.
- Provide quick-select presets for Last 24h, Last 7d, Last 30d, Last 90d, and Last 180d.
- Resolve every picker or preset action into explicit UTC ISO start/end values before calling existing APIs.
- Preserve existing timeframe validation, half-open `[start, end)` semantics, and range caps.
- Keep styling aligned with `apps/signal-ui/DESIGN.md` and route behavior aligned with `ui-wireframe.md`.

**Non-Goals:**

- No backend API, persistence, or runtime orchestration changes.
- No local-time default semantics for product data or backtest ranges.
- No automatic ingestion, backfill, repair, or data mutation when selecting a range.
- No replacement of unrelated filters, strategy parameters, or result tables.

## Decisions

### Use Ark UI Svelte primitives behind a product component

Create a shared component such as `apps/signal-ui/src/components/UtcDateRangePicker.svelte` plus small date/range utilities under `src/lib/`. Route pages should consume product-level props/events rather than depending directly on Ark UI state shapes.

- Rationale: Ark UI provides maintained Svelte-compatible primitives while a product wrapper isolates dependency details, UTC conversion, validation copy, and design-system styling.
- Alternative considered: wire Ark UI directly in each page. Rejected because it duplicates UTC/range logic and makes future time integration harder.
- Alternative considered: flatpickr. Rejected because Ark UI better fits current Svelte freshness, composition, range, preset, timezone, and time-integration needs.

### Store and submit UTC ISO strings at page boundaries

The shared picker should present calendar/range controls, but page state and API calls should continue to use explicit UTC ISO strings. The wrapper should normalize selected dates to UTC start/end instants and expose validation errors before callers can submit.

- Rationale: Existing API contracts and specs already use UTC-compatible timestamps and half-open ranges; preserving ISO strings at route boundaries reduces backend risk.
- Alternative considered: store date-library objects throughout the pages. Rejected because it couples route logic to a UI dependency and increases serialization mistakes.

### Resolve quick presets deterministically on activation

Quick presets should convert once, when selected, into explicit UTC `start` and `end` values and should not keep moving as wall-clock time changes. The data browser should prefer the selected availability/timeframe summary’s latest persisted candle end as the anchor when available; evaluation flows may anchor to the current UTC instant at click time unless a more specific deterministic dataset anchor is available.

- Rationale: Operators need convenient “last N” ranges, but deterministic backtests/data browsing require stable explicit ranges after selection.
- Alternative considered: continuously live-update “last N” ranges. Rejected because it can silently change replay inputs between review and submission.

### Enforce scope-specific constraints in the wrapper and callers

The shared picker should support min/max bounds, timeframe duration, and maximum interval count so `Data.svelte` can enforce the documented 10,000-candle cap and availability bounds before calling data APIs. `Evaluations.svelte` should enforce valid start/end and preserve the backend’s evaluation validation path.

- Rationale: Shared validation keeps UX consistent while route-level callers still own product-specific constraints.
- Alternative considered: rely only on backend rejection. Rejected because the current issue is operator-facing input friction and unclear invalid ranges.

## Risks / Trade-offs

- [Risk] Ark UI date primitives may require adapter glue for UTC/time granularity beyond date-only selection. → Mitigate by isolating Ark UI behind `UtcDateRangePicker` and keeping ISO conversion utilities tested.
- [Risk] Presets anchored to wall-clock time could make evaluation inputs seem less reproducible. → Mitigate by resolving once to visible UTC values and documenting that submitted ranges are explicit.
- [Risk] Data browser presets can exceed availability or interval limits for small timeframes. → Mitigate by clamping or disabling invalid presets with clear inline copy and preserving existing Load prevention.
- [Risk] Adding a dependency may affect bundle size and styling consistency. → Mitigate by using only needed Ark UI primitives and styling them through existing design-system classes/tokens.

## Migration Plan

1. Add the Ark UI Svelte dependency and any minimal date utility dependency required by the selected primitives.
2. Implement shared UTC range utilities and the product wrapper component with unit/component coverage for presets, ISO normalization, invalid ranges, and interval caps.
3. Replace free-entry UTC range controls in `Data.svelte`, preserving default availability behavior and data API request shape.
4. Replace free-entry evaluation range controls in `Evaluations.svelte`, preserving evaluation request payload shape.
5. Update UI documentation/wireframes for the new range picker behavior and run the `signal-ui` lint/test plus manual UI smoke flow.

Rollback is straightforward: remove the wrapper usage and dependency, then restore the previous text inputs while keeping the existing API request shapes.

## Open Questions

- Should evaluation presets anchor to current UTC at click time for v0, or should the evaluation page first discover latest local candle availability for the selected strategy scope?
- Should invalid data-browser presets be disabled, clamped to availability, or selectable with inline warnings? The implementation should choose one behavior and make it explicit in UI copy/tests.
