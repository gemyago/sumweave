## Context

The ledger model is intentionally transaction-driven:

```text
account balance = SUM(visible transaction.amountMinor)
```

The concern is the account-list/dashboard read path, not the ledger model. A balance read for 20 accounts should return about 20 aggregate rows, not every transaction needed to reconstruct those balances in application memory.

## Phase 1 Scope

Phase 1 should keep the system simple:

- add a SQL aggregate read for account balances grouped by `account_id`, with support for all-account and single-account reads
- expose booked and pending balances on account-list and account-detail responses
- move existing single-account balance reads such as `GetAccountBalance` to the aggregate path instead of listing transactions only to sum them
- avoid mutable account balance columns
- avoid materialized balance tables or background balance rebuild jobs
- avoid changing transaction write semantics

## Target Read Shape

The intended data flow is:

```text
GET /api/v1/finance/tenants/{tenantId}/accounts
  │
  ├─ load visible accounts for tenant
  │
  ├─ aggregate visible transactions by account/status in SQL
  │    booked  = SUM(amount_minor WHERE status = booked)
  │    pending = SUM(amount_minor WHERE status != booked)
  │
  └─ return account records with balances attached
```

The account-detail flow should use the same balance source:

```text
GET /api/v1/finance/tenants/{tenantId}/accounts/{accountId}
  │
  ├─ load the tenant-owned account
  │
  ├─ aggregate visible transactions for that account in SQL
  │
  └─ return the account record with balances attached
```

Standalone account-balance service reads should also use the same aggregate query rather than `ListTransactions` plus a Go loop.

The SQL shape should be equivalent to:

```sql
SELECT
  account_id,
  SUM(CASE WHEN status = 'booked' THEN amount_minor ELSE 0 END) AS booked_balance_minor,
  SUM(CASE WHEN status <> 'booked' THEN amount_minor ELSE 0 END) AS pending_balance_minor
FROM finance_transactions
WHERE tenant_id = ?
  AND hidden_at IS NULL
GROUP BY account_id;
```

Implementation does not need to use this exact SQL string if GORM or a dedicated query helper produces the same behavior.

For single-account reads, the query should add `account_id = ?` or otherwise restrict to the requested account. For dashboard period balances, the aggregate path may need an optional effective-date cutoff so dashboard account balances remain "as of period end" rather than always current.

## API Shape

The account-list and account-detail APIs should include native ledger balances per account. Suggested fields:

- `bookedBalanceMinor`
- `pendingBalanceMinor`

The account currency is already part of the account response, so no additional balance currency field is required unless implementation discovers the response shape needs it for clarity.

## Semantics To Preserve

Balance aggregation must match existing ledger behavior:

- visible transactions only
- hidden transactions excluded
- booked transactions contribute to booked balance
- pending transactions contribute to pending balance
- every transaction kind contributes by signed `amountMinor`
- refunds increase balance when positive but reduce expense in reporting only
- transfers affect the involved account balances by their signed amounts
- reconciliation and opening-balance transactions affect balances but stay excluded from income/expense reporting

## Index Consideration

Phase 1 may add a transaction index if tests or query inspection show it is useful for the aggregate path. The likely useful shape is tenant/account/status/hidden visibility, but this should stay minimal and aligned with SQLite local development plus PostgreSQL-oriented production.

Do not add broad speculative indexes unrelated to the account-balance aggregate read.

## Manual API-Level E2E Verification

This manual check is intentionally API-level only, similar in spirit to bank-linking verification. It should run against a local PM2-managed backend after schema migration.

Run setup:

```bash
go run ./apps/signal-foundry/cmd/signal-foundry db-migrate --env local
pm2 start ecosystem.config.js
pm2 status
```

Manual flow:

1. Authenticate as a local test user using the existing local API auth flow.
2. Create one finance tenant, or reuse a clean local tenant created for this check.
3. Create 2-3 accounts through the finance account API, for example checking, savings, and cash.
4. Record transactions through the finance transaction API for each account.
5. Include mixed transaction kinds and statuses:
   - regular booked positive income
   - regular booked negative expense
   - refund with positive amount
   - transfer out from one account
   - transfer in to another account
   - reconciliation or opening-balance adjustment
   - pending transaction
6. Call `GET /api/v1/finance/tenants/{tenantId}/accounts`.
7. Call `GET /api/v1/finance/tenants/{tenantId}/accounts/{accountId}` for at least two created accounts.
8. Verify each returned list item and detail response includes expected `bookedBalanceMinor` and `pendingBalanceMinor` values.
9. If any balance is wrong, record the failing request payloads, actual response, expected response, and suspected mismatch.
10. Fix the issue, restart/re-run migration only if the change requires it, and repeat the same API-level flow until the account-list and account-detail balances are correct.

Example expected math:

```text
checking:
  opening_balance +100000 booked
  regular income  +250000 booked
  regular expense  -40000 booked
  refund            +10000 booked
  transfer out      -30000 booked
  pending expense   -12000 pending

  expected bookedBalanceMinor  = 290000
  expected pendingBalanceMinor = -12000

savings:
  transfer in       +30000 booked
  reconciliation     +5000 booked

  expected bookedBalanceMinor  = 35000
  expected pendingBalanceMinor = 0
```

The manual check should report issues in a concise form:

```text
Issue:
  account: checking
  expected bookedBalanceMinor: 290000
  actual bookedBalanceMinor: 280000
  likely cause: refund excluded from account balance aggregate
  evidence: request/response snippets
```

## Non-Goals

- No materialized balance table.
- No account balance column maintained during transaction writes.
- No provider-balance snapshot reconciliation changes.
- No UI redesign.
- No change to income/expense reporting semantics beyond using the same account-balance aggregate where appropriate.
