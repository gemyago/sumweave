## 1. Venue Edge Foundation

- [x] 1.1 Add a narrow runtime venue edge package or equivalent runtime area for market-data source behavior.
- [x] 1.2 Define request/result types for instrument, candle, and trade reads using canonical domain types and `[start, end)` ranges.
- [x] 1.3 Keep vendor payloads, transport details, pagination tokens, and symbol mapping out of the data-layer contract.
- [x] 1.4 Add unit tests for venue edge validation, range handling, and canonical record construction.

## 2. Sandbox Venue

- [x] 2.1 Implement a seeded deterministic sandbox venue with stable instruments and configurable symbols/timeframes.
- [x] 2.2 Generate plausible candles and trades with UTC timestamps, non-negative prices/sizes, stable provenance, and stable source record identifiers.
- [x] 2.3 Add deterministic range filtering and minimal paging-like behavior for integration tests.
- [x] 2.4 Add sandbox tests covering reproducibility, different seeds, range boundaries, paging boundaries, and invalid requests.

## 3. Sandbox Data Integration

- [x] 3.1 Add an ingestion flow that consumes venue edge records and calls `runtime/data.IngestionService`.
- [x] 3.2 Add SQLite-backed integration tests that ingest sandbox instruments, candles, and trades into the data layer.
- [x] 3.3 Verify repeated sandbox ingestion is idempotent and read/replay results remain deterministically ordered.
- [x] 3.4 Verify sandbox ingestion preserves provenance, quality state, UTC timestamps, and `[start, end)` query behavior.

## 4. Real Venue Adapter With Mocked HTTP

- [x] 4.1 Select the first real venue target and capture the documented market-data endpoints, parameters, and response shapes used by v0.
- [x] 4.2 Implement a concrete real venue market-data adapter with injected HTTP client and configurable base URL.
- [x] 4.3 Add local HTTP test server coverage for documented success responses, pagination, non-success statuses, venue error payloads, and malformed payloads.
- [x] 4.4 Add mocked-HTTP-to-data integration tests that ingest real-adapter records through the data layer and read them back deterministically.
- [x] 4.5 Ensure default tests do not require live venue credentials, live network access, market availability, or external rate-limit state.

## 5. Documentation And Verification

- [x] 5.1 Update project or module documentation if the implementation introduces new commands, workflows, configuration, or architecture decisions.
- [x] 5.2 Confirm live real-venue E2E remains explicitly out of scope for v0 and capture any follow-up as a separate future change.
- [x] 5.3 Run focused runtime tests while developing the implementation.
- [x] 5.4 Run `make affected-lint-test` from the repository root before implementation completion and resolve all lint/test failures.
- [x] 5.5 Confirm AGENTS.md updates are unnecessary or apply any needed rule/convention changes before reporting implementation completion.
