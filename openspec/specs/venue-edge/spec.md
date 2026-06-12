# venue-edge Specification

## Purpose
TBD - created by archiving change add-venue-edge-v0. Update Purpose after archive.
## Requirements
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

The system SHALL validate concrete real venue adapters against published official API shapes using local test HTTP servers instead of live venue calls.

#### Scenario: Real venue adapters parse documented responses
- **WHEN** a local test HTTP server returns officially documented instrument, candle, or trade responses for a supported real venue adapter such as Binance or Hyperliquid
- **THEN** that adapter MUST map those responses into canonical domain records with stable provenance

#### Scenario: Real venue adapters handle paging and errors
- **WHEN** a local test HTTP server returns paged responses, non-success statuses, malformed payloads, or venue error payloads for a supported real venue adapter
- **THEN** that adapter MUST return deterministic results or explicit errors without partially hiding the venue failure

#### Scenario: Real venue adapters integrate with data layer through mocked HTTP
- **WHEN** a supported real venue adapter reads from a local test HTTP server and its records are ingested into the data layer
- **THEN** the data layer MUST persist and replay the records using the same canonical behavior verified for sandbox data

### Requirement: No Live Venue E2E In V0

The system SHALL keep live real-venue E2E testing out of the default v0 scope.

#### Scenario: Automated test suite avoids live network dependencies

- **WHEN** the default automated test suite runs
- **THEN** it MUST NOT require live venue credentials, live venue network access, market availability, or external rate-limit state

#### Scenario: Live venue behavior is not required for v0 completion

- **WHEN** venue edge v0 is considered complete
- **THEN** completion MUST depend on sandbox integration and mocked-HTTP real adapter tests, not live real-venue E2E tests

### Requirement: Transitional Multi-Adapter Real Venue Support

The system SHALL allow more than one concrete real market-data venue adapter to coexist behind the existing `venue-edge` canonical contract during venue-selection transitions.

#### Scenario: Multiple real adapters share the canonical boundary
- **WHEN** the runtime includes more than one concrete real venue adapter
- **THEN** each adapter MUST emit canonical `domain.Instrument`, `domain.Candle`, and `domain.Trade` records through the same venue-edge request/result contract without changing data-layer behavior

#### Scenario: Removing one adapter stays separable
- **WHEN** the team decides a concrete real venue adapter is slowing progress or no longer matches product direction
- **THEN** that adapter MUST be removable in a later change without requiring a new canonical data-layer contract

### Requirement: Hyperliquid Real Venue Adapter Coexists With Binance During Transition

The system SHALL support adding a Hyperliquid market-data adapter alongside the existing Binance adapter during the current venue-direction transition.

#### Scenario: Hyperliquid is added without immediate Binance removal
- **WHEN** Hyperliquid market-data support is introduced
- **THEN** the change MUST add Hyperliquid beside the existing Binance adapter rather than requiring Binance removal in the same change

#### Scenario: Hyperliquid remains within current venue-edge scope
- **WHEN** Hyperliquid is added to `runtime/venueedge`
- **THEN** the adapter MUST stay scoped to the current market-data venue-edge behavior and MUST NOT require order placement, fills, reconciliation, wallet signing, or execution-slice behavior

#### Scenario: Hyperliquid fixtures derive from official API docs
- **WHEN** Hyperliquid mocked-HTTP fixtures or committed test payloads are added
- **THEN** each fixture MUST be copied or minimally reduced from the published Hyperliquid API docs/examples for the exact request type under test and MUST NOT use ad hoc captured traffic as the normative source

### Requirement: Opt-In Live Real Venue Smoke

The system SHALL support a manual opt-in live smoke path for supported public market-data venue adapters without making live venue access part of the default automated suite.

#### Scenario: Live smoke stays out of default test lanes

- **WHEN** normal repository or module automated test targets run
- **THEN** they MUST NOT require live venue network access and MUST NOT execute the manual live smoke path implicitly

#### Scenario: Live-tagged tests still compile in regular validation

- **WHEN** regular repository or module validation runs
- **THEN** the system MAY compile `live`-tagged tests in an offline check, but it MUST NOT turn that compile-only verification into an implicit live network execution

#### Scenario: Live smoke exercises a real public adapter through runtime ingestion

- **WHEN** a developer intentionally runs the manual live smoke for a supported public venue adapter such as Hyperliquid
- **THEN** the smoke MUST read real public market data through the concrete adapter and ingest it through the existing `venueedge` ingestion flow

#### Scenario: Live smoke stays outside private venue behavior

- **WHEN** the first live smoke path is implemented for Hyperliquid
- **THEN** it MUST stay scoped to public read-only market-data behavior and MUST NOT require wallet signing, `approveAgent`, account provisioning, transfers, or trading actions

