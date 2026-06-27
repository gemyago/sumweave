## Why

Provider sync v2 is ready for another connector, but the only current providers depend on real external bank behavior. A deterministic synthetic provider gives the finance core a controllable provider for local development, sync orchestration, and provider-specific state evolution without adding a user-facing surface yet.

## What Changes

- Add a new product provider and technical connector named `synthetic` for generated bank data.
- Introduce a finance-internal core-only linking seam named `LinkConfiguredBankConnection`; in this change it supports only synthetic configured-account input and adds no HTTP, OpenAPI, or UI surface.
- Allow linking a synthetic bank connection from a provider-owned account configuration containing account names and currencies.
- Persist synthetic configured accounts with stable per-entry synthetic account keys so duplicate configured accounts with the same name and currency remain distinct and cannot collide.
- Generate provider account observations and one balance observation per configured account from that stored configuration.
- Generate synthetic transaction observations by exact normalized requested window:
  - normalize each requested window as the minimal UTC day span covering the half-open instant range `[start, end)`
  - first run for an unseen normalized window: 1 to 2 random booked transactions for each configured account for every UTC day in that normalized span
  - repeated run for the exact same normalized window: 1 to 3 random booked transactions for each configured account for the normalized window's last UTC day only
- Add dedicated synthetic-provider storage for configured accounts, normalized-window generation history, repeat counts, and per-account/day transaction sequence counters.
- Record synthetic generation history only on the successful synthetic fetch path for the normalized window; keep the common sync journal responsible for generic attempted-window outcome tracking.
- Keep the common provider sync v2 state journal generic; do not add provider-specific blobs to common journal rows for this iteration.
- Keep the iteration finance-core only: no finance UI, no public HTTP workflow, no user-facing provider setup screen, and no application-layer provider setup work yet.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: support a core-only synthetic bank provider with dedicated synthetic-provider state storage.

## Impact

- Affected code: `finance/domain`, `finance/internal/providers`, `finance/persistence`, and finance service/provider registration inside the finance module.
- Affected storage: finance-owned GORM auto-migrated synthetic-provider tables store account configuration and generation history.
- Affected behavior: synthetic provider fetches use dedicated synthetic storage to distinguish first-time and repeated normalized windows, emit account/balance/transaction/raw-payload observations, and leave existing provider sync journal semantics unchanged.
- Tests: focused finance module tests for synthetic storage round trips, duplicate configured-account identity handling, internal synthetic linking, balance observation coverage, normalized repeated-window behavior, synthetic fetch success-boundary persistence, and provider registration.
