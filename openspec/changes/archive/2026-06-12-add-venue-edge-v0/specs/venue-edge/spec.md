## ADDED Requirements

### Requirement: Isolated Venue Edge

The system SHALL isolate venue-specific market-data mechanics behind a narrow venue edge that emits canonical domain records for downstream deterministic slices.

#### Scenario: Venue edge emits canonical market data

- **WHEN** a venue source provides instruments, candles, or trades
- **THEN** the system MUST expose them as canonical `domain.Instrument`, `domain.Candle`, or `domain.Trade` records without exposing vendor payloads to the data layer

#### Scenario: Vendor mechanics stay at the edge

- **WHEN** a venue integration requires symbol mapping, pagination, HTTP transport, payload parsing, or source record identifiers
- **THEN** those mechanics MUST remain in the venue edge and MUST NOT become data-layer behavior

### Requirement: Deterministic Sandbox Venue

The system SHALL provide a sandbox venue that generates synthetic but deterministic market data for development and automated tests.

#### Scenario: Sandbox output is reproducible

- **WHEN** the sandbox venue is called with the same seed, venue, symbol, timeframe, and time range
- **THEN** it MUST produce the same canonical records with the same source record identifiers

#### Scenario: Sandbox output is venue-shaped

- **WHEN** the sandbox venue returns market data
- **THEN** the records MUST include stable venue identity, symbols, provenance source, provenance record identifiers, UTC timestamps, and non-negative price and size values

#### Scenario: Sandbox supports integration-relevant behavior

- **WHEN** automated tests request sandbox records across ranges or pages
- **THEN** the sandbox venue MUST support deterministic range filtering and enough paging-like behavior to exercise ingestion integration boundaries

### Requirement: Sandbox Data Integration

The system SHALL support automated integration tests that ingest sandbox venue output through the data layer.

#### Scenario: Sandbox records ingest into data layer

- **WHEN** sandbox venue records are passed through the venue ingestion flow
- **THEN** the data layer MUST persist canonical instruments, candles, and trades using its existing validation, idempotency, and provenance behavior

#### Scenario: Repeated sandbox ingestion is idempotent

- **WHEN** the same sandbox venue range is ingested more than once
- **THEN** the data layer MUST retain one canonical record per natural key or source record identifier according to the data-layer contract

#### Scenario: Ingested sandbox data is replayable

- **WHEN** tests read back an ingested sandbox range from the data layer
- **THEN** the returned records MUST be ordered deterministically and MUST respect `[start, end)` range semantics

### Requirement: Real Venue Adapter With Mocked HTTP

The system SHALL validate the first real venue adapter against documented HTTP API shapes using local test HTTP servers instead of live venue calls.

#### Scenario: Real venue adapter parses documented responses

- **WHEN** a local test HTTP server returns documented instrument, candle, or trade responses
- **THEN** the real venue adapter MUST map those responses into canonical domain records with stable provenance

#### Scenario: Real venue adapter handles paging and errors

- **WHEN** a local test HTTP server returns paged responses, non-success statuses, malformed payloads, or venue error payloads
- **THEN** the real venue adapter MUST return deterministic results or explicit errors without partially hiding the venue failure

#### Scenario: Real venue adapter integrates with data layer through mocked HTTP

- **WHEN** the real venue adapter reads from a local test HTTP server and its records are ingested into the data layer
- **THEN** the data layer MUST persist and replay the records using the same canonical behavior verified for sandbox data

### Requirement: No Live Venue E2E In V0

The system SHALL keep live real-venue E2E testing out of the default v0 scope.

#### Scenario: Automated test suite avoids live network dependencies

- **WHEN** the default automated test suite runs
- **THEN** it MUST NOT require live venue credentials, live venue network access, market availability, or external rate-limit state

#### Scenario: Live venue behavior is not required for v0 completion

- **WHEN** venue edge v0 is considered complete
- **THEN** completion MUST depend on sandbox integration and mocked-HTTP real adapter tests, not live real-venue E2E tests
