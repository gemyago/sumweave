## Context

The current finance implementation already has the right broad product shape: finance is a tenant-aware slice, fixture generation is service-backed, FX-backed reporting exists, and the UI exposes a dedicated finance workspace.

The remaining issues are narrower and product-default oriented:

- `finance.NewService` currently seeds only four default categories and no default tags for new tenants.
- The realistic fixture scenario creates only a handful of records, so local seeded environments do not look like a lived-in household ledger.
- Seeded environments can still surface missing-FX diagnostics immediately after fixture generation because the fixture path does not guarantee persisted rates for the seeded transaction dates.
- Finance UI tenant selection is still page-local instead of workspace-wide, finance dates are rendered as raw ISO strings, and the current-month dashboard mode leaves visible date inputs blank.

This change keeps the architecture intact and only strengthens the default catalog, demo data realism, and finance operator ergonomics.

## Goals / Non-Goals

**Goals**

- Seed a materially more useful default category and tag catalog for every new tenant.
- Keep the default catalog simple and flat enough for the current finance data model.
- Make the realistic fixture scenario seed a full year of activity with approximately 30-40 transactions per month.
- Ensure fixture generation persists the FX rates needed for seeded display-currency reporting without a follow-up sync step.
- Let operators work in one active tenant context at a time across finance routes.
- Render finance dates in a normal user-local format while keeping API and persistence semantics unchanged.
- Keep current-month dashboard mode and visible date controls synchronized.

**Non-Goals**

- No new backend concept of a server-side active tenant preference.
- No hierarchy migration for categories or tags; the current flat tenant-local catalog model stays in place.
- No redesign of finance reporting semantics, transfer semantics, or tenant membership permissions.
- No live external FX fetch requirement during seeding; fixture FX coverage should stay deterministic.
- No new budgeting methodology or per-user personalized category templates.

## Decisions

### 1. Seed a flat industry-aligned default catalog

Industry references point in the same general direction even though their data models differ:

- Monarch ships a broad household default category catalog spanning income, housing, utilities, food, transportation, health, shopping, travel, taxes, and transfer/system buckets, plus a small built-in tag set. It also treats tags as cross-cutting labels rather than replacements for categories.
- Quicken Simplifi similarly treats tags as cross-category reporting labels and keeps transfer-style behaviors distinct from ordinary expense categorization.

Because the current finance model supports flat tenant-local categories with only `income` or `expense` kind, this change should not import multi-level hierarchy directly. Instead, it should seed one deliberate flat catalog that covers the common household baseline without forcing excessive setup.

Canonical seeded categories:

- Income: `Paycheck`, `Bonus`, `Interest & Dividends`, `Business Income`, `Other Income`
- Expense: `Housing`, `Utilities`, `Groceries`, `Dining & Coffee`, `Transportation`, `Health & Medical`, `Insurance`, `Education & Childcare`, `Pets`, `Personal Care`, `Entertainment`, `Shopping`, `Home Improvement & Furnishings`, `Travel & Vacation`, `Gifts & Donations`, `Taxes & Fees`, `Debt Payments`, `Miscellaneous`

Canonical seeded tags:

- `Tax`
- `Reimburse`
- `Split`
- `Business`
- `Subscription`
- `Travel`

Transfer, reconciliation, and opening-balance behavior should remain transaction semantics, not seeded user categories.

### 2. Keep seeded defaults tenant-local after copy

New tenants should continue to receive copied defaults, not shared references. After creation, those categories and tags remain ordinary tenant-local catalog entries that users can rename, hide, or extend without future global seed changes mutating existing tenants.

### 3. Upgrade the realistic fixture scenario to a deterministic 12-month ledger

The default realistic fixture scenario should cover one full rolling year ending at the configured anchor month and should seed roughly 30-40 transactions per calendar month, for approximately 360-480 transactions total.

The seeded year should include:

- recurring income plus ordinary household spending
- multiple accounts and at least two currencies
- use of seeded default categories and default tags
- pending and booked activity
- refunds
- matched and unmatched transfers
- reconciliation/opening-balance records
- hidden records
- provider-origin and CSV/import-style examples
- representative finance job references/state where the current fixture path supports them

The transaction mix does not need to mimic a specific bank export, but it must feel like a realistic household ledger instead of a smoke-test skeleton.

### 4. Seed FX coverage as part of fixture generation

Fixture generation should persist the FX records required by its own seeded non-display-currency transactions. The implementation should use deterministic service-backed seeding rather than rely on a live post-seed sync.

Acceptance target:

- right after `signal-foundry finance fixtures generate ...`, seeded dashboard/reporting ranges should have the rates they need for converted totals
- seeded missing-FX diagnostics should represent intentional test cases only, not baseline fixture incompleteness

### 5. Keep active tenant selection client-side as a one-time workspace choice

The user operates on one finance tenant at a time, but this does not require a new backend preference record.

The simplest early-alpha shape is:

- if the user belongs to exactly one tenant, auto-select it as the workspace default and continue on the requested finance route, including finance detail deep links
- if the user belongs to multiple tenants and no active tenant is stored yet, require one explicit selection before loading tenant-scoped finance routes or finance-context deep links
- persist that choice in shared finance UI state/local storage
- reuse it across `/finance`, `/finance/accounts`, `/finance/accounts/:accountId`, `/finance/transactions`, `/finance/categories`, `/finance/connections`, `/finance/imports`, and `/finance/jobs/:jobId` until the operator changes it
- after an explicit tenant choice, keep the operator on the originally requested route instead of bouncing them to another finance page

The API can stay explicitly tenant-scoped per request.

### 6. Render finance dates in local format but keep wire semantics unchanged

Finance UI should stop showing raw ISO strings as the normal operator display format. User-facing finance dates and timestamps should render via a shared formatting helper using standard browser-local formatting.

This is display-only:

- API requests and responses stay ISO-compatible
- persistence stays UTC-first
- route/query semantics do not change

The formatter should cover both date-only and date-time cases so invites, dashboard periods, missing-FX diagnostics, schedules, and similar finance views stay consistent.

### 7. Synchronize current-month mode with visible date controls

The finance dashboard can keep the existing `preset` plus `startDate`/`endDate` API model, but the visible controls must no longer go blank when `current_month` is active.

When current-month mode is active, including initial page load and when the operator clicks `Current month`:

- the visible start/end date controls should display the current month's start and end dates
- the picker/calendar view should be anchored to the current month rather than stale custom state
- previous month, next month, and custom range actions should keep the visible controls synchronized with the active reporting window

## Risks / Trade-offs

- A flat category seed list is less expressive than Monarch- or Simplifi-style grouped hierarchies, but it matches the current finance model and avoids unnecessary schema work.
- Denser yearly fixture generation will increase test/setup data volume, so the generator should stay deterministic and linear rather than simulate every possible banking edge case.
- Localized date formatting can make tests brittle if locale handling is left implicit; the UI should centralize formatting so tests can assert consistent behavior.

## Migration Plan

1. Expand finance default catalog seeding and lock in the new baseline through finance-domain tests.
2. Upgrade realistic fixture generation volume and seed FX coverage needed by the generated ledger.
3. Update finance UI shared tenant selection behavior, finance date formatting helpers, and dashboard period controls.
4. Refresh the affected finance OpenSpec/UI docs and route/component tests, including `apps/signal-ui/ui-wireframe.md`, so the new defaults and workspace behavior are explicit.

## Open Questions

- No planning blockers are currently identified.
- Assumption: the active tenant preference remains client-side only for this change; no backend preference API is needed unless implementation uncovers a stronger cross-device requirement.
