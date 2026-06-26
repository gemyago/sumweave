## Why

The current finance slice works end to end, but several default and operator-facing behaviors are still too thin for regular use.

- New tenants only receive four default categories and no default tags, which is far below the category coverage commonly shipped by personal-finance tools.
- The realistic fixture scenario seeds only a few transactions and does not guarantee persisted FX coverage, so seeded demo environments do not resemble real usage and can still show missing conversion data right after seeding.
- Finance operators still re-select tenants route by route, see raw ISO dates in the UI, and get blank dashboard date inputs even when current-month mode is active.

These issues fit together as one finance polish change because they all affect the default out-of-the-box finance experience for seeded environments and day-to-day tenant operation.

## What Changes

- Expand the tenant-local default finance catalog from a minimal starter set to an industry-aligned flat household category baseline plus a small cross-cutting default tag set.
- Upgrade realistic finance fixture generation so the default scenario seeds one full year of activity with roughly 30-40 transactions per calendar month and automatically persists the FX coverage needed for seeded reporting.
- Improve finance workspace UX so finance tenant choice becomes a one-time workspace selection that defaults automatically when only one tenant exists, finance account and finance job deep links follow the same active-tenant rules, user-facing finance dates render in a standard local format, and current-month mode keeps the visible date controls synchronized with the active month.

## Capabilities

### Modified Capabilities

- `finance-management`: Expand the system default category and tag seed catalog used for new tenants.
- `finance-fixtures`: Make the realistic scenario materially denser and ensure seeded reporting FX data is available immediately.
- `finance-operator-ui`: Add one active tenant workspace context, replace raw ISO display dates with user-local formatting, and keep current-month controls visibly aligned with the active reporting month.

## Impact

- Affects `finance/`, especially tenant seeding defaults, realistic fixtures, reporting-seed coverage, and related tests.
- Affects `apps/signal-foundry/` fixture command coverage and any finance API/controller tests that depend on seeded defaults or seeded reporting behavior.
- Affects `apps/signal-ui/`, especially shared finance tenant selection state, finance account and finance job detail route behavior, finance date formatting helpers/components, the dashboard period controls, `ui-wireframe.md`, and finance route tests/docs.
- Updates OpenSpec finance specifications so the richer seed catalog, yearly demo data, FX availability, single-tenant workspace flow, and local-date UX are explicit acceptance criteria.
