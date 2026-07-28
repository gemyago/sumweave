# Finance Account Balances Manual E2E

Follow preparation steps in [README.md](./README.md) and get the API token before starting.
Run `mkdir -p tmp/manual-e2e` from the repository root before using the commands.

This guide is API-only. Do not use the UI for this check.

## 1. Create a fresh tenant using a unique tenant name

Use a unique tenant name on each run so the test data is easy to identify.

```bash
RUN_TAG=$(date +%Y%m%d-%H%M%S) && TENANT_NAME="finance-balances-e2e-${RUN_TAG}" && CREATE_STATUS=$(curl -sS -o tmp/manual-e2e/finance-balances-tenant-create.json -w "%{http_code}" -X POST "http://127.0.0.1:4501/api/v1/finance/tenants" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"name\":\"${TENANT_NAME}\",\"displayCurrency\":\"USD\",\"seedDefaults\":false}") && test "$CREATE_STATUS" = "200" && TENANT_ID=$(python3 -c 'import json; print(json.load(open("tmp/manual-e2e/finance-balances-tenant-create.json"))["id"])') && printf 'tenantId=%s\n' "$TENANT_ID"
```

Expected:

- create call returns `200`
- response includes the new tenant `id`

## 2. Create 2-3 accounts with the account create endpoint

Create checking, savings, and cash accounts:

```bash
CHECKING_STATUS=$(curl -sS -o tmp/manual-e2e/finance-balances-account-checking.json -w "%{http_code}" -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/accounts" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data '{"name":"Checking","kind":"manual","currency":"USD"}') && test "$CHECKING_STATUS" = "200" && CHECKING_ACCOUNT_ID=$(python3 -c 'import json; print(json.load(open("tmp/manual-e2e/finance-balances-account-checking.json"))["id"])') && SAVINGS_STATUS=$(curl -sS -o tmp/manual-e2e/finance-balances-account-savings.json -w "%{http_code}" -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/accounts" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data '{"name":"Savings","kind":"manual","currency":"USD"}') && test "$SAVINGS_STATUS" = "200" && SAVINGS_ACCOUNT_ID=$(python3 -c 'import json; print(json.load(open("tmp/manual-e2e/finance-balances-account-savings.json"))["id"])') && CASH_STATUS=$(curl -sS -o tmp/manual-e2e/finance-balances-account-cash.json -w "%{http_code}" -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/accounts" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data '{"name":"Cash","kind":"manual","currency":"USD"}') && test "$CASH_STATUS" = "200" && CASH_ACCOUNT_ID=$(python3 -c 'import json; print(json.load(open("tmp/manual-e2e/finance-balances-account-cash.json"))["id"])') && printf 'checking=%s\nsavings=%s\ncash=%s\n' "$CHECKING_ACCOUNT_ID" "$SAVINGS_ACCOUNT_ID" "$CASH_ACCOUNT_ID"
```

Expected:

- each create call returns `200`
- each response includes a new account `id`

## 3. Record a mixed transaction set via the transaction create endpoint

```bash
curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/transactions" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"accountId\":\"${CHECKING_ACCOUNT_ID}\",\"source\":\"manual\",\"kind\":\"opening_balance\",\"status\":\"booked\",\"amountMinor\":100000,\"currency\":\"USD\",\"description\":\"Opening balance\",\"effectiveAt\":\"2026-01-01T09:00:00Z\"}" && \
curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/transactions" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"accountId\":\"${CHECKING_ACCOUNT_ID}\",\"source\":\"manual\",\"kind\":\"regular\",\"status\":\"booked\",\"amountMinor\":250000,\"currency\":\"USD\",\"description\":\"Paycheck\",\"effectiveAt\":\"2026-01-02T09:00:00Z\"}" && \
curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/transactions" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"accountId\":\"${CHECKING_ACCOUNT_ID}\",\"source\":\"manual\",\"kind\":\"regular\",\"status\":\"booked\",\"amountMinor\":-40000,\"currency\":\"USD\",\"description\":\"Groceries\",\"effectiveAt\":\"2026-01-03T09:00:00Z\"}" && \
curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/transactions" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"accountId\":\"${CHECKING_ACCOUNT_ID}\",\"source\":\"manual\",\"kind\":\"refund\",\"status\":\"booked\",\"amountMinor\":10000,\"currency\":\"USD\",\"description\":\"Store refund\",\"effectiveAt\":\"2026-01-04T09:00:00Z\"}" && \
curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/transactions" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"accountId\":\"${CHECKING_ACCOUNT_ID}\",\"source\":\"manual\",\"kind\":\"transfer\",\"status\":\"booked\",\"amountMinor\":-30000,\"currency\":\"USD\",\"description\":\"Transfer to savings\",\"effectiveAt\":\"2026-01-05T09:00:00Z\"}" && \
curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/transactions" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"accountId\":\"${SAVINGS_ACCOUNT_ID}\",\"source\":\"manual\",\"kind\":\"transfer\",\"status\":\"booked\",\"amountMinor\":30000,\"currency\":\"USD\",\"description\":\"Transfer from checking\",\"effectiveAt\":\"2026-01-05T09:00:00Z\"}" && \
curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/transactions" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"accountId\":\"${SAVINGS_ACCOUNT_ID}\",\"source\":\"manual\",\"kind\":\"reconciliation\",\"status\":\"booked\",\"amountMinor\":5000,\"currency\":\"USD\",\"description\":\"Statement adjustment\",\"effectiveAt\":\"2026-01-06T09:00:00Z\"}" && \
curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/transactions" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"accountId\":\"${CHECKING_ACCOUNT_ID}\",\"source\":\"manual\",\"kind\":\"regular\",\"status\":\"pending\",\"amountMinor\":-12000,\"currency\":\"USD\",\"description\":\"Pending card charge\",\"effectiveAt\":\"2026-01-07T09:00:00Z\"}" && printf 'transaction set created\n'
```

Expected booked and pending math:

- checking booked `290000`, pending `-12000`
- savings booked `35000`, pending `0`

## 4. Verify account list and detail balances

Verify the account list:

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/accounts" -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Verify account detail for checking and savings:

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/accounts/${CHECKING_ACCOUNT_ID}" -H "Authorization: Bearer ${ACCESS_TOKEN}" && printf '\n' && curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/accounts/${SAVINGS_ACCOUNT_ID}" -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Expected:

- checking has `bookedBalanceMinor=290000` and `pendingBalanceMinor=-12000`
- savings has `bookedBalanceMinor=35000` and `pendingBalanceMinor=0`
- cash has `bookedBalanceMinor=0` and `pendingBalanceMinor=0`

## 5. If anything is wrong, report it

If any balance is incorrect, report:

- request payloads
- actual response
- expected response
- suspected mismatch
