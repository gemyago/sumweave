## Why

The ledger already supports manual transfer pairing and excludes matched internal
transfers from income and expense reporting, but users must still identify every
pair themselves. Phase 0 adds one deterministic automatic matcher so simple
movements between tracked tenant accounts are recognized after bank sync or on an
explicit historical run without changing uncertain transactions.

## What Changes

- Add a finance-owned transfer-matching service that loads one complete eligible
  ledger slice, finds mutually unique equal-and-opposite same-currency pairs within
  72 hours in memory, and saves accepted pairs atomically.
- Add `transfer_matching_excluded boolean NOT NULL DEFAULT false` to finance
  transactions, preserve pair-owned fields through ordinary saves and provider
  refreshes, and silently exclude both legs after manual unlinking while retaining
  broader manual linking.
- Add an authenticated explicit matching API that publishes
  `finance.transfer-matching.explicit.v1`, returns its appdispatch message ID as a
  future `finance.transfer-matching` job ID, and never executes matching inline.
- Add an independent `finance.transfer-matching.v1` subscriber to the existing
  committed bank-sync-window event, with no automatic job projection or dependency
  on classification success.
- Extend worker startup, bounded `--once` draining, error joining, and shutdown to
  own the observed worker plus both ordinary enrichment routers safely.
- Add a tenant-ledger **Match transfers** range action with local-calendar bounds,
  existing job feedback, terminal ledger refresh, and matching-specific success
  and partial-failure copy.
- Document and manually verify deterministic explicit, automatic, correction,
  reporting, retry, and headed browser flows against isolated PostgreSQL.

## Capabilities

### New Capabilities

- `transfer-matching`: Eligibility, deterministic mutual-uniqueness matching,
  range scope, atomic pair persistence, exclusions, explicit observed work,
  automatic committed-window handling, diagnostics, and worker lifecycle.

### Modified Capabilities

- `finance-management`: Ordinary transaction saves and provider refreshes must
  preserve pair-owned state, and manual unlinking must retain user data while
  excluding both legs from future automatic matching.
- `finance-operator-ui`: The tenant ledger gains explicit transfer matching with
  local-date selection, scope explanation, existing job feedback, and terminal
  refresh behavior.

## Impact

- Finance transaction domain/model mappings, GORM auto-migration, transaction
  upsert ownership, provider merge mapping, dedicated pair persistence, matcher,
  focused service composition, and ledger pair workflows.
- Backend semantic commands, observed and ordinary handlers, OpenAPI-first route
  and generated code, controller wiring, jobs worker lifecycle, and module docs.
- Finance SPA client/range logic, transaction ledger UI/tests, wireframe, and
  manual E2E runbooks.
- PostgreSQL is the only affected database. `runtime/` and third-party dependencies
  are unchanged.
