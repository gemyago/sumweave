## ADDED Requirements

### Requirement: Raw Payload Browser Read Models
The data layer SHALL expose read-only raw payload browser models that are safe for backend/API consumption without exposing GORM persistence models or raw response bodies in list results.

#### Scenario: Raw payload metadata list is filtered and ordered
- **WHEN** a caller lists raw venue payload metadata with venue and optional symbol, asset class, timeframe, half-open time range, ingestion run ID, entity hint, endpoint, request type, limit, and cursor filters
- **THEN** the system MUST return matching metadata rows only, ordered deterministically by received time ascending and raw payload identity ascending
- **AND** the list result MUST NOT include raw response body bytes or a response body preview

#### Scenario: Raw payload metadata list paginates deterministically
- **WHEN** the raw payload metadata list has more matches than the bounded requested limit
- **THEN** the system MUST return at most the requested limit and an opaque cursor that continues from the same stable order

#### Scenario: Raw payload detail returns bounded body preview
- **WHEN** a caller requests a raw payload detail by identity
- **THEN** the system MUST return raw payload metadata, response body size in bytes, a deterministic bounded response body preview, and whether the preview was truncated
- **AND** the system MUST read body bytes through the raw payload blob abstraction rather than from a database body column

#### Scenario: Missing raw payload detail is reported
- **WHEN** a caller requests a raw payload identity that is not persisted
- **THEN** the system MUST return a not-found result without fabricating metadata or body preview content

#### Scenario: Candle-linked raw payload metadata is read by full canonical candle key
- **WHEN** a caller requests raw payload evidence linked to a canonical candle using venue, symbol, asset class, timeframe, half-open candle start/end, provenance source, and provenance identity
- **THEN** the system MUST return only raw payload metadata linked to that exact provenance-bearing canonical candle, ordered deterministically
- **AND** the system MUST NOT require callers to depend solely on UI-facing database row identity
- **AND** the system MUST NOT fall back to matching only by venue, symbol, asset class, timeframe, and candle time range when provenance is omitted

#### Scenario: Candle-linked raw payload lookup rejects missing provenance
- **WHEN** a caller requests raw payload evidence linked to a canonical candle without a non-empty provenance source or without a non-empty provenance identity
- **THEN** the system MUST return a deterministic validation error rather than selecting among potentially ambiguous candle natural keys
