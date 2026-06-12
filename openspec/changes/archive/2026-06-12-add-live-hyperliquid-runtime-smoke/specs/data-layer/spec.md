## ADDED Requirements

### Requirement: Opt-In Live Ingestion Smoke Through SQLite

The system SHALL support a manual opt-in smoke that persists real public venue data through the canonical data-layer ingestion and read path using an ephemeral SQLite store.

#### Scenario: Live venue data persists through the canonical path

- **WHEN** a developer intentionally runs the runtime live smoke against a supported public venue adapter such as Hyperliquid
- **THEN** the smoke MUST ingest canonical instruments, candles, or trades into SQLite through the existing ingestion and persistence services rather than bypassing the data layer

#### Scenario: Live smoke validates canonical readback

- **WHEN** live venue records are persisted by the smoke
- **THEN** the smoke MUST read them back through the canonical read services and verify structural invariants such as canonical venue identity, UTC-normalized timestamps, and deterministic ordering semantics where applicable

#### Scenario: Live smoke avoids brittle market snapshot assertions

- **WHEN** the live smoke validates persisted records from a real venue
- **THEN** it MUST focus on canonical persistence and readback invariants rather than exact record counts or fixed market values that are expected to change across runs
