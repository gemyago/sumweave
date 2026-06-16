## ADDED Requirements

### Requirement: Shared UTC Date Range Picker
The Signal Foundry UI SHALL provide a reusable Ark UI-backed date range picker for operator workflows that require historical UTC time ranges.

#### Scenario: Picker exposes explicit UTC range values
- **WHEN** an operator selects or edits a date range through the shared picker
- **THEN** the picker MUST expose explicit UTC-compatible `start` and `end` ISO timestamp values to the consuming route
- **AND** the resulting range MUST represent half-open `[start, end)` semantics for API requests
- **AND** the picker MUST NOT convert product ranges to browser-local-time semantics

#### Scenario: Picker preserves time precision required by workflows
- **WHEN** a route provides existing UTC timestamps with time-of-day precision or uses a preset such as Last 24h
- **THEN** the picker MUST preserve or emit UTC time-of-day precision needed by that workflow rather than silently reducing the range to local calendar dates

#### Scenario: Picker uses product styling and accessible controls
- **WHEN** the picker is rendered in the Signal Foundry UI
- **THEN** it MUST use existing design-system styling conventions and semantic labels for range start, range end, calendar navigation, and preset controls

### Requirement: Deterministic Quick Range Presets
The shared date range picker SHALL support deterministic quick-select presets for common product ranges.

#### Scenario: Required presets are available
- **WHEN** a consuming route enables quick range presets
- **THEN** the picker MUST offer Last 24h, Last 7d, Last 30d, Last 90d, and Last 180d

#### Scenario: Preset resolves to an explicit UTC range
- **WHEN** an operator activates a quick range preset
- **THEN** the picker MUST resolve the preset once into visible explicit UTC `start` and `end` values
- **AND** the `start` value MUST equal the resolved `end` anchor minus the preset duration
- **AND** the selected preset MUST NOT continue moving as wall-clock time advances

#### Scenario: Preset anchor can be workflow-specific
- **WHEN** a consuming route supplies a deterministic UTC anchor such as the latest persisted candle end for a selected scope
- **THEN** the picker MUST use that anchor for preset resolution
- **AND** when no workflow-specific anchor is supplied the picker MUST use the current UTC instant at the time the operator activates the preset

### Requirement: Range Constraint Validation
The shared date range picker SHALL allow consuming routes to enforce workflow-specific UTC range constraints before API calls are made.

#### Scenario: Invalid ranges are rejected inline
- **WHEN** the selected or edited range has missing timestamps, non-UTC-compatible timestamps, or `start >= end`
- **THEN** the picker or consuming route MUST show inline validation
- **AND** the consuming route MUST NOT submit the invalid range to its API

#### Scenario: Timeframe and bound constraints are enforced
- **WHEN** a consuming route provides minimum/maximum UTC bounds, a timeframe duration, or a maximum interval count
- **THEN** the picker MUST allow the route to reject or prevent ranges outside those constraints before submission
- **AND** the inline validation MUST identify the violated constraint in operator-facing terms
