## 1. Shared Provider Sync V2 Foundation

- [x] 1.1 Add synthetic provider identifiers and finance registry/profile registration; follow TDD flow by adding registry/profile tests first, then add `synthetic` provider and connector constants plus static registry wiring inside the finance module.
- [x] 1.2 Invoke connector fetch from the v2 window executor; follow TDD flow by expanding executor tests first, then resolve the connector, call `Fetch`, and return batch-derived stats/issues through the shared executor path before provider-specific end-to-end wiring.
- [x] 1.3 Keep fetched observations compatible with downstream v2 seams; follow TDD flow by adding focused executor/planner tests first, then ensure account, balance, transaction, provider-original, fingerprint, and raw-payload observations from `Fetch` satisfy existing diff/apply planning contracts.

## 2. Synthetic Storage And Core Linking

- [x] 2.1 Add dedicated synthetic-provider persistence; follow TDD flow by adding synthetic store round-trip tests first, then implement finance-owned GORM models, auto-migration registration, and a narrow store keyed by bank connection.
- [x] 2.2 Model synthetic state; follow TDD flow by adding state serialization tests first, then implement a typed versioned envelope for configured accounts, stable synthetic account keys, normalized-window history, repeat counters, and per-account/day sequence counters.
- [x] 2.3 Introduce the finance-internal `LinkConfiguredBankConnection` seam; follow TDD flow by adding core linking tests first, then validate non-empty account name/currency input, assign a stable synthetic account key per configured entry, persist provider-owned synthetic config, and create an active synthetic connection without adding HTTP or UI exposure.

## 3. Synthetic Fetch Generation

- [x] 3.1 Implement synthetic first-window generation; follow TDD flow by adding connector fetch tests first, then generate provider account observations, one balance observation per configured account, identifying raw payloads, and 1 to 2 booked transactions per configured account for each UTC day in an unseen normalized requested window.
- [x] 3.2 Implement exact normalized-window repeated generation; follow TDD flow by adding repeated-fetch tests first, then normalize requested windows as the minimal UTC day span covering `[start, end)`, match repeats by exact normalized UTC start/end-exclusive boundaries, and generate 1 to 3 booked transactions per configured account only for the normalized window's last UTC day.
- [x] 3.3 Persist synthetic generation history only on the successful synthetic fetch path; follow TDD flow by adding fetch persistence tests first, then load and save the versioned synthetic state envelope through the dedicated synthetic store so successful fetches advance normalized-window history/repeat counters/sequence counters while failed fetches leave generation history unchanged.

## 4. Finance-Module Composition

- [x] 4.1 Wire synthetic into finance provider composition only where core code needs it; follow TDD flow by adding composition tests first, then register the synthetic connector and `LinkConfiguredBankConnection` support without adding finance UI controls, public HTTP routes, or OpenAPI provider enums.
