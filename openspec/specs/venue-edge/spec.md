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

### Requirement: Hyperliquid Info Raw Evidence Capture
The venue edge SHALL capture raw Hyperliquid public `/info` request/response evidence for supported market-data reads before canonical normalization while preserving canonical returned records for downstream consumers.

#### Scenario: Raw payload identities use read-result metadata
- **WHEN** Hyperliquid raw capture is enabled for a supported public market-data read
- **THEN** the adapter MUST expose the persisted raw payload identity through optional read-result metadata on the existing result struct
- **AND** `MarketDataVenue` method signatures MUST remain unchanged
- **AND** canonical `domain.Instrument`, `domain.Candle`, and `domain.Trade` records MUST NOT include raw payload identity or body fields

#### Scenario: Hyperliquid meta capture precedes instrument normalization
- **WHEN** the Hyperliquid perps adapter sends a `/info` request with type `meta`
- **THEN** the adapter MUST record raw request/response evidence before mapping the response into canonical instruments
- **AND** the read result MUST still expose canonical `domain.Instrument` records for downstream consumers

#### Scenario: Hyperliquid candle snapshot capture precedes candle normalization
- **WHEN** the Hyperliquid perps adapter sends a `/info` request with type `candleSnapshot` for an instrument, timeframe, and half-open time range
- **THEN** the adapter MUST record raw request/response evidence including the request payload hash, request metadata, HTTP status, response timestamps, response body hash, response body reference, entity hint, instrument, timeframe, and time range before mapping the response into canonical candles
- **AND** the read result MUST still expose canonical `domain.Candle` records for downstream consumers

#### Scenario: Hyperliquid recent trades capture uses deterministic read window only
- **WHEN** the Hyperliquid perps adapter sends a `/info` request with type `recentTrades` for the supported single latest-window read
- **THEN** the adapter MUST record exactly that raw request/response evidence before mapping the response into canonical trades
- **AND** the adapter MUST NOT introduce recent-trade paging, historical backfill, or extra venue calls to manufacture deterministic history in v0

#### Scenario: Hyperliquid raw capture appends on repeated fetches
- **WHEN** a caller repeats the same supported Hyperliquid public market-data read
- **THEN** the adapter MUST cause a new raw payload evidence record to be persisted for the new HTTP exchange rather than overwriting the prior raw payload record
- **AND** normalized canonical records MUST continue to use their existing deterministic idempotency rules when persisted downstream

#### Scenario: Hyperliquid error response is captured before returning error
- **WHEN** Hyperliquid returns a non-success HTTP status or a malformed body for a supported public `/info` request
- **THEN** the adapter MUST record the request/response evidence that is available before returning the venue error
- **AND** the error path MUST NOT expose raw response bodies through canonical domain records

#### Scenario: Hyperliquid capture is optional for existing callers
- **WHEN** the Hyperliquid perps adapter is constructed without a raw evidence recorder
- **THEN** the adapter MUST continue to perform supported public market-data reads and return canonical records with the existing behavior
- **AND** raw payload metadata on read results MUST be empty when no capture recorder is configured
- **AND** automated tests MUST be able to verify capture behavior through mocked HTTP without live Hyperliquid network access

#### Scenario: Hyperliquid capture accepts optional ingestion-run context
- **WHEN** a lineage-aware ingestion flow provides an ingestion run identity to the configured Hyperliquid raw evidence recorder
- **THEN** the adapter MUST pass that identity as raw evidence metadata for persistence validation
- **AND** direct adapter reads without a run identity MUST remain valid standalone captures

