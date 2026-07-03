## Why

Finance account balances are ledger-derived from transaction `amountMinor` values. That is the right source-of-truth model, but the current read path for dashboard/account balance presentation loads visible tenant transactions into application memory and sums them in Go. For a first real user with roughly 20+ accounts and multiple years of transaction history, SQLite/PostgreSQL can aggregate this cheaply, while moving full transaction history across the DB boundary for an account list is unnecessary work and the wrong long-term read shape.

## What Changes

- Add a dedicated account-balance aggregate read path that computes booked and pending balances in SQL for one account or many accounts.
- Return ledger-derived balances on the tenant account-list API so account lists do not need to fetch or iterate full transaction history.
- Add or update the tenant account-detail API so `GET /api/v1/finance/tenants/{tenantId}/accounts/{accountId}` returns the same ledger-derived balance fields.
- Keep transactions as the ledger source of truth; do not add mutable `accounts.balance` fields or materialized balance tables in phase 1.
- Preserve existing balance semantics: visible transactions only, booked and pending separated, every transaction kind contributes by signed `amountMinor`, and hidden transactions are excluded.
- Use the aggregate read path anywhere account detail, account list, standalone account-balance reads, or dashboard account balances need account-level balances instead of fetching transactions only to sum them in application memory.
- Add an API-level manual e2e verification flow that creates multiple accounts, records mixed transaction kinds, lists accounts, checks balances, reports any issue, fixes, and re-runs until correct.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: Strengthen account-list and account-detail reads so tenant account responses include ledger-derived booked and pending balances computed by a SQL aggregate read model rather than application-side full-history iteration.

## Impact

- Affects `finance/`, especially persistence/read-store shape for account balance aggregation and finance-domain service tests around ledger-derived account balances.
- Affects `apps/signal-foundry/`, especially finance account-list/account-detail API response models, OpenAPI route schema, controller mapping, generated route/model files, and API/controller tests.
- Does not affect `apps/signal-ui/` unless the UI chooses to display newly returned account-list balances immediately.
- Does not introduce stored account balances, materialized balance tables, or write-time balance mutation logic.
