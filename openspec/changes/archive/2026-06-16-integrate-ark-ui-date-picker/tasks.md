## 1. Dependency And UTC Range Utilities

- [x] 1.1 Add the Ark UI Svelte dependency and isolate product-facing imports; follow TDD flow by first adding or adjusting a minimal UI build/import test, then wiring the dependency, then verifying the test and lockfile changes.
- [x] 1.2 Implement shared UTC range and preset utilities for ISO normalization, half-open ranges, workflow anchors, and preset duration math; follow TDD flow by writing failing utility tests for Last 24h/7d/30d/90d/180d and invalid ranges before implementation.

## 2. Shared Range Picker Component

- [x] 2.1 Build the shared Ark UI-backed `UtcDateRangePicker` component with design-system styling and accessible labels; follow TDD flow by writing component tests for rendered controls, visible UTC values, and emitted ISO start/end values before implementation.
- [x] 2.2 Add configurable validation for required ranges, UTC compatibility, min/max bounds, timeframe duration, and maximum interval count; follow TDD flow by writing constraint tests before implementing validation and inline error rendering.

## 3. Historical Data Browser Integration

- [x] 3.1 Replace `Data.svelte` free-entry UTC range text boxes with the shared range picker while preserving default availability loading and explicit Load behavior; follow TDD flow by updating data-page tests for no API call until Load before implementation.
- [x] 3.2 Wire data-browser presets to selected availability/timeframe latest persisted candle end and enforce availability bounds plus the 10,000-interval cap; follow TDD flow by adding tests for preset anchoring, rejected oversized ranges, and stable visible UTC values before implementation.

## 4. Evaluation Runner Integration

- [x] 4.1 Replace `Evaluations.svelte` free-entry UTC range text boxes with the shared range picker while preserving the existing evaluation request payload; follow TDD flow by adding tests for submitted explicit UTC start/end values before implementation.
- [x] 4.2 Add evaluation quick-select presets that resolve once to explicit UTC ranges and reject invalid or empty ranges inline; follow TDD flow by writing tests for stable preset resolution and blocked invalid submissions before implementation.

## 5. Documentation And UI Behavior Alignment

- [x] 5.1 Update `apps/signal-ui/ui-wireframe.md` and related UI documentation for the new UTC range picker behavior; follow TDD flow by first codifying the user-visible behavior in route/component tests, then updating docs and copy to match the implemented behavior.
