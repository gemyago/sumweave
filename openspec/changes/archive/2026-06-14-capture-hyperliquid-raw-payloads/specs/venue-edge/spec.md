## ADDED Requirements

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
