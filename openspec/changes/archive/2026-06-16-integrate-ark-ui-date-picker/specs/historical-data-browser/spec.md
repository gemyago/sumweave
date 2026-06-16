## MODIFIED Requirements

### Requirement: Protected Historical Data Browser UI
The operator UI SHALL provide a protected `#/data` route for browse-first, read-only historical normalized candle browsing.

#### Scenario: Authenticated operator opens data route
- **WHEN** an authenticated operator opens `#/data`
- **THEN** the UI MUST load normalized candle availability before requiring manual candle filter entry
- **AND** the UI MUST render available venue, symbol, and asset class entries with timeframe/range summaries when availability exists
- **AND** the authenticated nav MUST include Data, Chat, and Providers entries

#### Scenario: Authenticated default route opens data
- **WHEN** an authenticated operator opens the application without an explicit route
- **THEN** the UI MUST land on `#/data` rather than the chat route
- **AND** explicit `#/chat` navigation MUST remain available

#### Scenario: Post-login fallback route opens data unless a route was requested
- **WHEN** an unauthenticated user completes login after opening the app root, an empty hash, or any flow without a saved explicit destination
- **THEN** the UI MUST navigate to `#/data`
- **AND** when the unauthenticated user originally requested an explicit protected route such as `#/chat` or `#/data`, successful login MUST return to that requested route instead of overriding it with the data default

#### Scenario: Unauthenticated operator is redirected
- **WHEN** an unauthenticated user opens `#/data`
- **THEN** the UI MUST redirect to login using the existing protected route behavior

#### Scenario: Default availability selection loads candles
- **WHEN** the availability response includes a default selection
- **THEN** the UI MUST select that entry and call `GET /api/v1/data/candles` with the default venue, symbol, asset class, timeframe, start, and end from the availability response
- **AND** the UI MUST render the returned normalized candles without requiring the operator to press Load first
- **AND** the UI MUST NOT call the broad `GET /api/v1/data/raw-payloads` metadata list as part of this initial availability/default-candle auto-load

#### Scenario: Default candle selection is populated when data exists
- **WHEN** the default or selected candle slice returns one or more candles
- **THEN** the UI MUST select a deterministic default candle from the returned slice
- **AND** the selected candle state MUST include the candle's venue, symbol, asset class, timeframe, start/end, provenance source, and provenance identity for linked evidence lookup
- **AND** default candle selection MUST use the same linked raw evidence behavior as manual candle selection

#### Scenario: No availability avoids guessed candle queries
- **WHEN** the availability response has no browseable normalized candle entries
- **THEN** the UI MUST show an empty state explaining that no normalized candle data is available
- **AND** the UI MUST NOT call the candle endpoint with guessed venue, symbol, timeframe, or time range values

#### Scenario: Selecting an availability entry begins browsing
- **WHEN** an operator selects an available venue, symbol, and asset class entry
- **THEN** the UI MUST use that item’s per-entry default slice and load candles for that exact scope
- **AND** selecting an entry MUST NOT start ingestion, backfill, repair, or any other mutation

#### Scenario: UTC range picker replaces free-entry range text boxes
- **WHEN** an operator edits the candle query range after selecting an availability entry
- **THEN** the UI MUST provide the shared UTC-aware range picker instead of requiring free-entry UTC start and end text boxes
- **AND** the picker MUST show the selected UTC `start` and `end` values that will be sent to the data APIs
- **AND** changing the range in the picker MUST NOT call the data APIs until the operator activates Load

#### Scenario: Data browser presets use deterministic candle anchors
- **WHEN** an operator chooses Last 24h, Last 7d, Last 30d, Last 90d, or Last 180d for a selected availability entry
- **THEN** the UI MUST resolve the preset against that entry and timeframe’s latest persisted candle end when available
- **AND** the resolved UTC range MUST remain visible and stable until the operator changes it again
- **AND** the UI MUST respect the entry’s earliest available candle start and the selected timeframe’s range limits before enabling Load

#### Scenario: Filter edits remain explicit and validated
- **WHEN** an operator edits timeframe or UTC start/end after selecting an availability entry and activates Load
- **THEN** the UI MUST call the candle and raw payload metadata APIs with the selected scope and edited query params, disable Load while loading, and render summary counts, a candlestick chart from normalized candle OHLC values, and a raw payload metadata table
- **AND** invalid UTC ranges, ranges outside the selected availability bounds, or ranges exceeding the documented 10,000-interval server cap MUST show inline validation and MUST NOT call the data APIs

#### Scenario: Empty and error states are visible
- **WHEN** candles or raw payload metadata responses are empty
- **THEN** the UI MUST show appropriate empty states without clearing unrelated successful data unnecessarily
- **AND** API errors MUST be shown using alert semantics

#### Scenario: Raw payload detail drawer shows bounded preview
- **WHEN** an operator selects a raw payload metadata row
- **THEN** the UI MUST fetch raw payload detail and open an accessible detail drawer with metadata, hashes, body reference, instrument/timeframe/range hints, bounded response body preview, and truncation indication

#### Scenario: Selected candle shows linked raw evidence
- **WHEN** an operator selects a candle from the reliable v0 candle selection path
- **THEN** the UI MUST call the candle-linked raw payload API with the selected candle's `provenanceSource` and `provenanceIdentity` and render linked raw payload evidence or an empty evidence state
