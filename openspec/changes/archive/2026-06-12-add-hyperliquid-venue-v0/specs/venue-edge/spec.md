## ADDED Requirements

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

## MODIFIED Requirements

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
