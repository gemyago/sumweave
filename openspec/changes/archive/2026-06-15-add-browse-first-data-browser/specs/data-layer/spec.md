## ADDED Requirements

### Requirement: Normalized Candle Availability Read Models
The data layer SHALL expose read-only normalized candle availability models derived from persisted canonical candle rows so operator surfaces can discover browseable historical candle data without relying on raw payload metadata or live venue symbol lists.

#### Scenario: Availability request contract is fixed and validated
- **WHEN** a caller lists normalized candle availability
- **THEN** the request MUST support only optional exact `venue`, `symbol`, and `assetClass` filters plus `limit` and opaque `cursor` pagination controls
- **AND** the default limit MUST be 50 entries, the maximum accepted limit MUST be 200 entries, and pagination MUST apply to top-level venue + symbol + asset class entries after filters are applied
- **AND** unsupported filters, unsupported venues or asset classes, non-canonicalizable symbols, invalid limits, or invalid cursors MUST return deterministic validation errors

#### Scenario: Availability includes only persisted normalized candle entries
- **WHEN** a caller lists normalized candle availability
- **THEN** the system MUST return only top-level venue, symbol, and asset class entries that have at least one persisted canonical candle row in at least one timeframe
- **AND** the system MUST NOT include symbols that exist only as raw payload metadata or only as live venue/reference data

#### Scenario: Availability item shape summarizes deterministic candle ranges
- **WHEN** a persisted candle availability item is returned
- **THEN** the item MUST represent exactly one venue + symbol + asset class entry
- **AND** the item MUST include a non-empty `timeframes` collection containing only persisted timeframe summaries for that entry
- **AND** each timeframe summary MUST include timeframe, earliest persisted candle start, latest persisted candle end, and persisted candle count
- **AND** timeframe summaries within an item MUST be ordered by timeframe duration ascending
- **AND** the system MUST NOT synthesize, interpolate, or imply candles for missing intervals inside that range

#### Scenario: Availability ordering and pagination are stable
- **WHEN** a caller lists normalized candle availability repeatedly against the same persisted candle data
- **THEN** the system MUST return top-level entries in the same order using each entry's maximum latest available candle end descending, then venue ascending, symbol ascending, and asset class ascending as deterministic tie-breakers
- **AND** cursor pagination MUST continue from the same stable top-level entry order without duplication or omission across pages

#### Scenario: Per-entry default browse slice is deterministic and bounded
- **WHEN** a returned availability item contains one or more persisted timeframe summaries
- **THEN** the item MUST include a default slice derived from the item’s timeframe summary with latest persisted candle end descending and timeframe duration ascending as deterministic tie-breakers
- **AND** the default half-open range MUST end at that timeframe summary’s latest persisted candle end
- **AND** the default range start MUST be the later of that timeframe summary’s earliest persisted candle start and `latestEnd - 500 * duration(timeframe)`
- **AND** the default slice MUST NOT request more than 500 timeframe intervals or fill missing intervals

#### Scenario: First-page default selection is deterministic and bounded
- **WHEN** availability is requested without a cursor and the filtered result contains at least one persisted top-level entry
- **THEN** the result MUST include a default selection that mirrors the first returned entry’s venue, symbol, asset class, default timeframe, default start, and default end
- **AND** the default selection MUST be valid as an exact candle read scope without requiring additional discovery
- **AND** availability results requested with a cursor MUST omit the top-level default selection

#### Scenario: Empty availability has no default browse slice
- **WHEN** no persisted canonical candle rows exist for the requested availability scope
- **THEN** the system MUST return an empty availability result without a default selection
