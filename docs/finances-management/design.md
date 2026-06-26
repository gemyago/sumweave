# Finance Management

The finance management slice extends Signal Foundry with personal finance
tracking for one or more user-owned tenants. The initial proof of concept proved
that we can fetch account and transaction data from Enable Banking / PKO and
monobank. The next step is to turn that POC into a productized module that can
be implemented safely and evolved independently.

Relevant POC notes:

- [Enable Banking / PKO POC](../../apps/signal-foundry/doc/financial-poc/enable-banking-pko.md)
- [monobank POC](../../apps/signal-foundry/doc/financial-poc/monobank.md)
- [Frankfurter FX API](https://frankfurter.dev/)
- [NBP Web API](https://api.nbp.pl/en.html)
- [ECB reference rates](https://www.ecb.europa.eu/stats/policy_and_exchange_rates/euro_reference_exchange_rates/html/index.en.html)

Related docs:

- [Finance Provider Sync Architecture](../finance-provider-sync-architecture.md)

The design below is the implementation target for the first production-quality
finance slice. The repository is still early in development, so API and storage
shape can change as needed while implementing this slice.

## Early Alpha Policy

This project is early alpha. Breaking API changes, destructive migrations, data
loss during incompatible migration changes, and other breaking changes are
acceptable while the design is still settling.

Implementation should optimize for the clean target architecture, not backward
compatibility. Do not add compatibility layers, legacy migrations, dual-write
paths, or transition mechanisms unless the user explicitly asks for them.

## Goals

- Let users create or join finance tenants.
- Let tenant members manage accounts, transactions, categories, and tags.
- Link bank accounts from the UI and synchronize them from day one.
- Support both on-demand sync and scheduled background sync from day one.
- Support Enable Banking / PKO and monobank first, using the successful POC.
- Support manual accounts and manual transaction entry.
- Support CSV import for historical data from other systems.
- Provide tenant dashboards with reporting-period controls, income vs expense
  summaries, and category breakdowns.
- Keep this slice architecturally independent from the trading runtime.

## Non-Goals For The First Implementation

- Tenant roles and permissions. All tenant members are equal for now.
- Cross-tenant account sharing.
- Payment initiation or money movement.
- Budgeting, forecasting, or automated advice.
- Fully automated transaction categorization. The first implementation supports
  manual categorization, with data retained for later automation.
- Per-tenant encryption keys. Use one system-configured symmetric key first.

## Architecture

### Module Boundary

Create a new root Go module named `finance/`.

`finance/` is independent from `runtime/`. It owns the finance domain, business
services, connector interfaces, persistence models, migrations, and sync logic.
The existing backend application in `apps/signal-foundry/` depends on both
`runtime/` and `finance/` and wires the finance HTTP API into the same process.

Expected dependency direction:

```text
apps/signal-foundry/ -> finance/
apps/signal-foundry/ -> runtime/
apps/signal-ui/      -> apps/signal-foundry/ HTTP API
finance/ must not import runtime/
```

The finance module must not import trading runtime packages. If we later regroup
product modules under a broader `domains/` folder, this module can move then.

### System Jobs Architecture

Finance needs background work, but jobs are a system-level concern rather than a
finance-only concept. The repository already has a durable jobs foundation under
`apps/signal-foundry/internal/jobs` for historical market-data backfills:

- persisted job records with status, input, result, errors, requester, attempts,
  worker ID, timestamps, and idempotency key
- a polling/wakeable worker
- startup recovery for stale running jobs
- retry limits
- HTTP endpoints for create/list/get
- app-level DI/configuration and shutdown integration

That foundation should be promoted into a generic app-level jobs substrate before
adding finance jobs. The current implementation is useful but still product
specific: its `Job` model stores historical-backfill input/result shapes, and
its worker directly knows the historical-backfill runner. The next version
should keep the durable pattern while making job input/result payloads generic
JSON and dispatching execution through registered typed handlers.

Recommended system shape:

- `apps/signal-foundry` owns the jobs runtime because it owns the process,
  configuration, HTTP API, lifecycle, and worker binary modes.
- The API process enqueues jobs and exposes job status, but does not execute
  durable jobs inline.
- Durable jobs execute in a separate worker process mode using the same Go
  binary, for example a future `signal-foundry jobs worker` command.
- Local development may run API and worker under the same PM2 ecosystem, but
  production should run them as separate processes/pods.
- Product modules expose services and, where useful, typed job input/result
  contracts; `apps/signal-foundry` registers app-level job handlers that call
  those product services. Product modules must not import the app jobs runtime.
- The jobs table remains system-level, table-prefixed with the app's jobs prefix,
  not finance-prefixed.
- Job types are namespaced by product, for example
  `finance.bank_connection_sync`, `finance.fx_rates_sync`,
  `finance.csv_import`, and `data.historical_raw_candle_backfill`.
- The job store uses generic `input_json`, `result_json`, and optional
  `progress_json`, while typed handlers validate and decode their own payloads.
- Job records include status, timestamps, worker ID, attempts, max attempts,
  requester, source, idempotency key, correlation ID, and a sanitized error.
- Supported statuses should be at least `queued`, `running`, `succeeded`,
  `failed`, and `canceled`. `scheduled` can stay separate as schedule metadata
  rather than a job status.
- Workers claim jobs transactionally, execute handlers, persist result/error,
  and recover stale `running` jobs on startup.
- Manual and scheduled jobs both create durable job records. Scheduled work
  should not run as hidden goroutines with no visible job history.
- Kubernetes CronJobs should provide the production scheduling tick. A CronJob
  runs a short-lived command that scans schedules and enqueues due durable jobs;
  it should not perform the long-running product work itself.
- A lightweight schedule registry in the database stores recurring definitions,
  due times, and last enqueue state. The Kubernetes CronJob is only the external
  timer, while the database remains the source of truth for dynamic schedules.
- The UI should expose a generic jobs/status surface that product screens can
  deep-link to for sync/import diagnostics.

Initial finance job types:

- `finance.bank_connection_sync`: sync one bank connection, optionally scoped to
  one provider account and date range.
- `finance.fx_rates_sync`: sync FX rates for configured currency pairs/date
  windows.
- `finance.csv_import`: import a confirmed CSV preview, including account upsert
  and transaction import.
- `finance.account_import`: import accounts from CSV when the user wants an
  accounts-only migration.

Scheduling decisions:

- Bank connection sync schedules are stored per active connection and can also
  be triggered on demand from the UI.
- FX-rate sync has a global schedule and can also be triggered manually from an
  admin/diagnostics UI.
- In Kubernetes, use standard CronJobs for the scheduler tick, for example a
  periodic `signal-foundry jobs enqueue-due` command.
- In local development, the same enqueue-due command can be run manually or by
  PM2 if recurring local sync is needed.
- CSV imports are explicit user-triggered jobs after preview/confirmation.
- Account imports are explicit user-triggered jobs after preview/confirmation.

The first finance implementation should refactor the existing jobs package into
the generic target shape directly. Because this is early alpha, no compatibility
layer is needed for the current historical-backfill-only job schema.

### Backend API

Keep one API application for now: extend `apps/signal-foundry/` with finance API
routes. The finance route tree should be distinct from runtime routes, for
example under `/api/v1/finance/...`.

The backend app should remain a thin composition layer:

- authentication and process-level middleware stay in `apps/signal-foundry/`
- finance business rules stay in `finance/`
- provider-specific HTTP details stay behind finance connector implementations
- generated API glue can live in the backend app if that matches current app
  routing conventions

### UI

Reuse `apps/signal-ui/`, but create a clearly distinct finance area rather than
mixing finance screens into trading/operator workflows. The UI must support:

- a top-level Finance navigation area with tenant-aware routes
- tenant selection, create, invite, and join flows
- dashboard with period controls, charts, KPI cards, and supporting tables
- accounts list, account detail, manual account creation, account import, and
  linked-account attachment
- bank-linking flows for Enable Banking and monobank
- bank connection list/detail with re-authentication, last sync, next scheduled
  sync, and sync job history
- transaction list with filtering, sorting, edit, hide/delete, pending status,
  transfer linking, category assignment, and tag assignment
- category and tag management
- CSV transaction import flow with preview, mapping, validation, and confirmed
  async import job
- CSV account import flow with preview, mapping, validation, and confirmed async
  import job
- generic job status/detail views reusable by finance sync/import screens
- a minimal admin/diagnostics area for system jobs, FX sync, provider health,
  and operational status

Recommended finance routes:

- `#/finance`: tenant dashboard for the selected tenant
- `#/finance/tenants`: tenant switcher and tenant management
- `#/finance/accounts`: account list
- `#/finance/accounts/:accountId`: account detail and ledger view
- `#/finance/connections`: bank connections and sync health
- `#/finance/transactions`: transaction workspace
- `#/finance/categories`: category and tag management
- `#/finance/imports`: CSV import entrypoint and import history
- `#/finance/jobs/:jobId`: finance-focused job detail, backed by the generic
  jobs API
- `#/admin`: minimal admin/diagnostics landing page
- `#/admin/jobs`: generic jobs list with filters by status, type, source, and
  created time
- `#/admin/jobs/:jobId`: generic job detail with sanitized input, progress,
  result, error, attempts, and worker metadata
- `#/admin/finance/fx`: FX-rate coverage, last sync, missing rates, provider
  selection/status, and manual FX sync trigger
- `#/admin/finance/providers`: bank provider status, connection health summary,
  rate-limit diagnostics, and failed sync visibility

UI principles:

- Prefer summary cards and focused detail routes over dense split-pane screens.
- Keep charts explanatory, not decorative; every chart should have a nearby
  tabular/list view for exact values.
- Always show currency and conversion state when display-currency totals are
  derived from FX rates.
- Make pending, hidden, transfer, refund, and reconciliation states visually
  explicit in transaction lists.
- Treat bank-linking and import flows as step-by-step workflows with clear
  validation and recovery paths.

Minimal admin UI scope:

- The admin UI can be simple and utilitarian for the first implementation.
- There are no tenant roles yet, so it can be visible to authenticated users in
  early alpha unless a separate admin role is introduced later.
- Admin screens must not display decrypted secrets or raw provider payloads by
  default.
- Admin screens should focus on operational clarity: failed jobs, missing FX
  rates, stale schedules, re-authentication needs, and manual retry/sync actions.

### Persistence

Use the same persistence approach as the rest of the product direction:

- GORM
- explicit migrations
- SQLite for local development
- PostgreSQL for production when needed
- UTC-first timestamps

Use `finance_` table prefixes for the first implementation. This keeps table
ownership obvious and works consistently across SQLite and PostgreSQL. A future
PostgreSQL-only deployment may additionally schema-scope finance tables, but the
portable baseline is table prefixes.

Persistence models must stay separate from domain models. Provider raw payloads
must be stored for audit, debugging, and future connector bugfixes.

### Secrets And Sensitive Data

Provider secrets and session credentials are stored in the same database for
now, encrypted with a system-configured symmetric key.

Required rules:

- never log raw tokens, private keys, session credentials, or decrypted secrets
- store encrypted provider credentials in dedicated fields/tables
- retain enough metadata to know whether a link needs re-authentication
- design the encryption boundary so per-tenant key wrapping can be added later
- treat raw provider payloads as sensitive personal data even when not secret

## Development And Test Data

Create a CLI command that generates realistic finance fixture data for local
development, manual UI testing, and automated smoke tests.

Recommended command location:

- command lives in `apps/signal-foundry/cmd/signal-foundry` because that is the
  existing product binary and it can use the same config, auth, database, and DI
  wiring as the API
- generation logic lives in `finance/` as a domain-level scenario generator
- the CLI must call finance application/domain services instead of writing
  directly to GORM models or database tables

Suggested command shape:

```text
signal-foundry finance fixtures generate \
  --tenant-name "Demo Household" \
  --from 2026-01-01 \
  --to 2026-03-31 \
  --profile household-alpha \
  --seed 12345
```

Required fixture behavior:

- generate a few months of data by default
- support deterministic generation through a seed
- create one or more users and tenants where useful for local testing
- create multiple accounts across different currencies
- create manual accounts, linked-account-shaped accounts, and imported accounts
- create categories and tags using the same services as normal tenant creation
- create regular income and expense transactions
- create pending and booked transactions
- create refunds that reduce category expense
- create matched internal transfers and unmatched/external transfers
- create reconciliation/opening-balance transactions
- create hidden/deleted transactions
- create transactions with user-edited provider-original fields
- create enough volume to exercise pagination, filtering, charts, and dashboard
  performance without making local testing slow
- create FX-rate records for the generated currencies and period
- optionally create job records for succeeded, running, queued, failed, and
  canceled finance jobs so jobs/admin UI states are easy to test

The fixture generator should expose reusable scenario functions from `finance/`
so automated tests can construct the same shapes without shelling out to the CLI.
Because this is early alpha, the fixture schema may change freely with the
domain model.

## Domain Model

### User

A person authenticated in the system. A user may belong to one or more finance
tenants.

### Tenant

The tenant is the root finance entity. It represents a finance unit such as a
household, but product copy must not call it "family" because future tenants may
represent other units.

Requirements:

- users can create or join tenants
- users join tenants through an invitation code/link flow
- all existing tenant members may invite new members while roles are out of
  scope
- tenants have a user-friendly name
- tenants have one display currency
- all high-level overviews are displayed in the tenant display currency
- all tenant members are equal for now

### Account

An account belongs to exactly one tenant. There is no cross-tenant account
sharing.

Required fields:

- tenant
- user-friendly name
- currency
- optional description
- account kind/source: manual, linked bank account, CSV/imported, or similar
- optional bank-linking metadata
- current sync/balance metadata where available

Users can enter transactions for any account, linked or not. Existing manual
accounts can be selected during bank linking so the bank link attaches to an
already-created account instead of always creating a duplicate account.

### Account Balance

Account balances are ledger-driven from transactions. Users cannot directly
mutate a balance field. If a user needs to correct a balance, the system creates
a visible reconciliation transaction.

Bank providers may return balance snapshots. These snapshots are observations
from the provider, not direct ledger mutations. If a linked account's ledger
balance must be aligned to a provider balance, create a system reconciliation or
opening-balance transaction so the ledger remains explainable.

### Bank Connection

A bank connection represents provider-level linking credentials and sessions.
One connection may expose one or more provider accounts.

Initial providers:

- Enable Banking / PKO: UI redirect and strong customer authentication flow.
- monobank: UI flow where the user supplies a personal token.

Connections must track:

- provider
- tenant
- credential/session state
- access expiry or re-authentication state when available
- last sync status
- last successful sync time
- provider raw metadata needed for support/debugging

### Transaction

A transaction is any financial operation recorded against an account.

Transactions may be:

- synchronized from a bank provider
- manually entered
- imported from CSV
- system-created for reconciliation/opening balance

Core concepts:

- direction: income or expense
- kind: regular, refund, transfer, or reconciliation
- status: pending/hold or booked/settled
- source: manual, provider, CSV import, system
- original amount and currency
- tenant display-currency amount for reporting, derived from stored FX rates
- category: zero or one category
- tags: zero or more tags
- raw provider/import payload where applicable

Users may edit transaction fields without artificial product limits. For synced
transactions, keep the provider-original values and raw payload separately from
user-edited values so future syncs do not silently destroy user corrections.

### Deleting And Hiding Transactions

Provider-synced transactions should not be hard-deleted. "Delete" in the UI
should hide/exclude the transaction from user views and reports while keeping the
provider record for sync idempotency and debugging.

Manual or CSV-imported transactions may also use soft deletion for consistency.

### Refunds

Refunds are visible transactions in the transaction list. For reporting, a
refund reduces expense in the assigned category rather than appearing as normal
income.

Example: a grocery refund assigned to `Groceries` reduces `Groceries` expense
for the reporting period.

### Transfers

Transfers can link two counterpart transactions.

Reporting behavior:

- matched internal transfers are excluded from income/expense reports
- unmatched or external transfers are reported by direction
- external incoming transfer is income unless the user classifies it otherwise
- external outgoing transfer is expense unless the user classifies it otherwise

The UI should make it possible to link counterpart transfer transactions after
sync/import, because different providers may expose each side differently.

### Reconciliations

Reconciliation transactions are visible in the transaction list. They adjust the
ledger balance, but they do not count as income or expense in dashboard reports.

### Pending Transactions

Pending/hold transactions should be imported and shown. They must be handled as
non-final provider observations:

- visible in transaction lists with a clear pending status
- excluded from settled income/expense totals by default
- optionally included in UI previews/filters
- upserted or matched to the final booked transaction when the provider settles
  it

Sync logic must expect provider behavior to vary. Some providers keep stable
transaction IDs from pending to booked; others may emit a different final
transaction. Store provider IDs and enough normalized fields to support safe
matching by provider, account, amount, currency, date/time, status, and
description when needed.

### Categories And Tags

Use a flat category list plus tags for the first implementation.

Reasoning:

- a transaction having exactly one category keeps dashboard reporting simple
- tags provide flexible detail such as `restaurants`, `groceries`, `work-trip`,
  `kid`, or `subscription`
- tags can cross-cut categories without forcing awkward category trees
- a future category hierarchy can still be added if reporting needs become more
  formal

Requirements:

- each transaction may have zero or one category
- each transaction may have zero or more tags
- system default categories are copied into each new tenant
- tenant category/tag changes are tenant-local
- later changes to system defaults must not mutate existing tenant categories
- the system must not separate income and expense categories at the system level

## Reporting And Currency Conversion

Each tenant has a display currency. All high-level reports must be shown in that
currency.

Store both original transaction amounts and display-currency reporting amounts.
Reporting conversion must be reproducible, so use persisted FX-rate records
rather than recalculating historical reports from only live rates.

Use Frankfurter as the default FX data source for the first implementation. It
is free, public, requires no API key, is open source/self-hostable, supports
historical rates and time series, and tracks central-bank sources with provider
attribution. Prefer provider-specific queries when we need official reference
rates for auditability.

FX source policy:

- default source: Frankfurter public API
- preferred query mode: request only the required base/quote pairs and date
  windows, and persist returned provider attribution when available
- preferred official provider filter: ECB where the currency pair is covered
- PLN-focused fallback/source option: NBP Web API for official Polish central
  bank rates, especially when tenant display currency is PLN
- direct fallback/source option: ECB reference-rate downloads for EUR-based
  rates when the Frankfurter service is unavailable or when direct official ECB
  ingestion is preferred
- provider abstraction must allow changing the source without changing reporting
  or transaction storage

Required FX behavior:

- run a scheduled FX-rate sync job
- allow manual refresh if needed
- store rates by base currency, quote currency, date/time, provider, and value
- use transaction-date rates for historical reporting where possible
- clearly surface missing rates instead of silently producing misleading totals

For MVP, dashboards should still show native account balances alongside
display-currency summaries when that improves clarity.

## Dashboard Requirements

The tenant dashboard must support:

- current month as the default reporting period
- month-based navigation with the current month selected by default
- quick previous/next month controls so users can easily move to the previous
  month or return to the current month
- adjustable reporting period presets: current month, previous month, last 3
  months, last 6 months, this year, and previous year
- custom date range selection when presets are not enough
- tenant display-currency totals for settled income, settled expenses, net cash
  flow, and current account balances where FX is available
- visible warning/diagnostic state for missing FX rates
- pending transaction preview separate from settled totals
- native account balances and display-currency summaries where FX is available

Show as charts:

- income vs expenses over time for the selected period, using a line or grouped
  bar chart depending on the period granularity
- expense breakdown by category, using a donut/pie chart only for top-level
  quick review
- net cash flow trend over time
- optional account balance trend when historical balances can be reconstructed
  reliably from transactions and reconciliations

Show as non-chart summaries:

- KPI cards for income, expenses, net cash flow, pending total, and missing FX
  count/state
- account balance summary cards grouped by account/currency
- last sync and next scheduled sync status
- alerts for stale bank connections, failed jobs, missing rates, and import
  issues

Show as tables/lists:

- category breakdown table with exact amounts, percentages, refund adjustments,
  and transaction counts
- account balances table with native balance, display-currency balance, last
  provider snapshot, and last sync
- recent transactions list
- pending transactions list
- failed or recently completed finance jobs list
- FX coverage/missing-rate diagnostics when rates are incomplete

The transaction list must support:

- filtering by account
- filtering by category
- filtering by tags
- filtering by date range
- filtering by status
- filtering by amount
- filtering by description/search text
- sorting by date, amount, category, account, and status
- editing transactions
- hiding/deleting transactions

## Bank Sync

Bank linking and synchronization are required from day one.

Required sync modes:

- on-demand sync from the UI
- scheduled background sync
- retryable sync jobs with visible status
- last successful sync timestamp per connection/account

Sync must be idempotent. The system should deduplicate using provider,
connection, account, provider transaction ID, and when necessary a fallback
fingerprint based on stable transaction fields.

The sync layer must preserve:

- normalized account records
- normalized transaction records
- pending vs booked status
- provider balance snapshots
- raw provider payloads
- provider IDs and account identifiers needed for future sync

Provider rate limits and range limits must be respected. monobank range chunking
and rate-limit behavior from the POC should be carried forward into the
production connector.

## CSV Import

CSV import must support historical migration from other systems.

Required behavior:

- choose the target tenant
- import accounts from CSV
- import transactions for multiple accounts in one CSV
- read the target account from each CSV row, using an account name column
- upsert accounts by tenant plus account name when the user confirms
- optionally map account currency, description, opening balance, and external
  source identifiers from account CSVs or transaction CSV rows
- map CSV columns to transaction fields
- preview rows before import
- import original amount/currency, date, description, category, tags, and notes
- create missing categories/tags only when the user confirms
- create or update missing accounts only when the user confirms
- deduplicate repeat imports where possible
- store import run metadata and raw row payloads for audit/debugging

Import flow:

1. User selects tenant and import type: accounts, transactions, or combined.
2. User uploads CSV.
3. System detects headers and proposes column mapping.
4. User confirms mappings for account name, account currency, transaction date,
   amount, currency, description, category, tags, and optional notes/source IDs.
5. System previews rows, validation errors, duplicate candidates, and accounts,
   categories, or tags that would be created.
6. User confirms import.
7. System enqueues a durable import job.
8. UI shows import job progress, result summary, and any rejected rows.

Account import must be available even without transactions. This supports
migrating an existing account list before importing historical ledger data.

## Suggested API Areas

Exact route names can be finalized during implementation, but the API should
cover these areas:

- tenants: create, list joined tenants, update tenant metadata
- tenant members: list members, create invite, accept invite
- accounts: create, list, update, hide/delete, attach bank link
- bank connections: start link, finish link, list, re-authenticate, disconnect
- sync jobs: trigger, list status, inspect failures
- transactions: list/filter/sort, create, update, hide/delete, link transfers
- categories: list, create, update, hide/delete
- tags: list, create, update, hide/delete
- dashboard: reporting summaries and breakdowns
- FX rates: sync status and diagnostics
- CSV imports: account import preview/confirm/status, transaction import
  preview/confirm/status, combined import preview/confirm/status
- jobs: generic list/get/cancel/retry where supported, with finance-specific
  filters/deep links
- fixture generation: local/dev-only command path for realistic finance demo
  data; no public HTTP API is required for fixture generation

## Implementation Slices

Recommended implementation order:

1. Refactor the existing app jobs package into the generic durable jobs
   substrate and migrate historical backfills to it.
2. Create `finance/` module skeleton, domain types, persistence setup, and
   migrations.
3. Add finance fixture generator services and the app CLI command so UI/API
   development can use realistic data early.
4. Add tenants, memberships, accounts, categories, and tags.
5. Add transaction storage, transaction list/edit/hide behavior, and dashboard
   queries without bank sync.
6. Add FX-rate storage/sync jobs and display-currency reporting.
7. Add provider connection model and encrypted credential storage.
8. Productize Enable Banking / PKO linking and sync.
9. Productize monobank token linking and sync.
10. Add scheduled bank sync jobs and on-demand UI-triggered sync.
11. Add CSV account import, transaction import, preview, and confirm flows.
12. Complete UI workflows for dashboard, accounts, transactions, categories,
    tags, bank linking, sync status, and CSV import.

## Design Decisions Captured

- Users create or join tenants.
- Tenant members are equal; roles may come later.
- Accounts belong to exactly one tenant.
- Jobs are a generic app-level system; the app registers handlers that call
  finance services.
- Jobs execute in separate worker process mode, not inline in the API process.
- Kubernetes CronJobs provide the production scheduler tick for enqueueing due
  durable jobs.
- Scheduled work creates visible durable job records.
- Early alpha means implementation may break APIs and migrations freely to reach
  the clean target design.
- Provider-synced transaction deletion means hide/exclude, not hard delete.
- Synced transactions can be user-edited, with provider originals retained.
- Pending transactions are imported, shown, and excluded from settled totals by
  default.
- Refunds reduce expense in their assigned category.
- Matched internal transfers are excluded from reports; external transfers
  report by direction.
- Reconciliation transactions are visible but excluded from income/expense
  reports.
- Categories are flat; tags provide flexible detail.
- Income and expense do not have separate system category trees.
- Tenant display-currency reporting uses persisted FX rates.
- Frankfurter is the default FX-rate source; NBP and ECB are supported source
  options/fallbacks.
- `finance/` is a root Go module, independent from `runtime/`.
- Provider raw payloads are stored for audit and debugging.
- Provider secrets use one system symmetric key for the first implementation.
- Bank sync supports both manual and scheduled sync from day one.
- Enable Banking uses redirect/SCA linking; monobank uses token entry.
- CSV imports select tenant only; accounts are imported or matched from CSV
  account names so one import can cover multiple accounts.
- Finance fixture generation is an app CLI command backed by `finance/` domain
  services, not direct database writes.
