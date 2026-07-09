# Synthetic Provider Flow Manual E2E

Follow preparation steps in [README.md](./README.md) and get the API token before starting.

This guide stays on the public HTTP API only. It replaces the old temporary Go helper with the real synthetic start/configure/finish endpoints.

## 1. Create a fresh finance tenant

Use a unique tenant name on each run.

```bash
RUN_TAG=$(date +%Y%m%d-%H%M%S) && TENANT_NAME="synthetic-provider-e2e-${RUN_TAG}" && CREATE_STATUS=$(curl -sS -o /tmp/synthetic-provider-tenant-create.json -w "%{http_code}" -X POST "http://127.0.0.1:4501/api/v1/finance/tenants" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"name\":\"${TENANT_NAME}\",\"displayCurrency\":\"USD\"}") && test "$CREATE_STATUS" = "200" && TENANT_ID=$(python3 -c 'import json; print(json.load(open("/tmp/synthetic-provider-tenant-create.json"))["id"])') && printf 'tenantId=%s\n' "$TENANT_ID"
```

Expected:

- `200`
- response includes a new tenant `id`

## 2. Start synthetic setup

```bash
START_STATUS=$(curl -sS -o /tmp/synthetic-provider-start.json -w "%{http_code}" -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections/link-redirect/start" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data '{"provider":"synthetic","callbackUrl":"http://127.0.0.1:5173/#/finance/connections"}') && test "$START_STATUS" = "200" && STATE=$(python3 -c 'import json; data=json.load(open("/tmp/synthetic-provider-start.json")); assert data["provider"] == "synthetic"; assert "#/finance/connections/synthetic?state=" in data["authorizationUrl"]; print(data["state"])') && printf 'state=%s\n' "$STATE"
```

Expected:

- `200`
- response includes `provider="synthetic"`
- response includes a local `authorizationUrl` under `#/finance/connections/synthetic?state=...`
- response includes non-empty `state`

## 3. Confirm the pending state is empty before save

```bash
GET_STATE_STATUS=$(curl -sS -o /tmp/synthetic-provider-state-initial.json -w "%{http_code}" "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections/synthetic-link-states/${STATE}" -H "Authorization: Bearer ${ACCESS_TOKEN}") && test "$GET_STATE_STATUS" = "200" && python3 -c 'import json; data=json.load(open("/tmp/synthetic-provider-state-initial.json")); assert data["provider"] == "synthetic"; assert data["state"]; assert data["configuredAccounts"] == []; assert data["canFinish"] is False; print("canFinish=false")'
```

Expected:

- `200`
- `configuredAccounts` is empty
- `canFinish=false`

## 4. Save duplicate configured accounts

Use duplicate `name` and `currency` values to confirm the API keeps them distinct with stable synthetic account keys.

```bash
SAVE_STATUS=$(curl -sS -o /tmp/synthetic-provider-state-saved.json -w "%{http_code}" -X PUT "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections/synthetic-link-states/${STATE}" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data '{"configuredAccounts":[{"name":"Synthetic Checking","currency":"USD"},{"name":"Synthetic Checking","currency":"USD"}]}') && test "$SAVE_STATUS" = "200" && python3 -c 'import json; data=json.load(open("/tmp/synthetic-provider-state-saved.json")); items=data["configuredAccounts"]; assert len(items) == 2; assert items[0]["key"]; assert items[1]["key"]; assert items[0]["key"] != items[1]["key"]; assert data["canFinish"] is True; print("configuredKeys=" + ",".join(item["key"] for item in items))'
```

Expected:

- `200`
- response includes two configured accounts
- each configured account has non-empty `key`
- duplicate rows keep distinct keys
- `canFinish=true`

## 5. Reload and re-save the pending state

```bash
GET_RELOADED_STATUS=$(curl -sS -o /tmp/synthetic-provider-state-reloaded.json -w "%{http_code}" "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections/synthetic-link-states/${STATE}" -H "Authorization: Bearer ${ACCESS_TOKEN}") && test "$GET_RELOADED_STATUS" = "200" && python3 -c 'import json; saved=json.load(open("/tmp/synthetic-provider-state-saved.json")); print(json.dumps({"configuredAccounts": saved["configuredAccounts"]}))' > /tmp/synthetic-provider-state-resave-payload.json && RESAVE_STATUS=$(curl -sS -o /tmp/synthetic-provider-state-resaved.json -w "%{http_code}" -X PUT "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections/synthetic-link-states/${STATE}" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data @/tmp/synthetic-provider-state-resave-payload.json) && test "$RESAVE_STATUS" = "200" && python3 -c 'import json; saved=json.load(open("/tmp/synthetic-provider-state-saved.json"))["configuredAccounts"]; reloaded=json.load(open("/tmp/synthetic-provider-state-reloaded.json"))["configuredAccounts"]; resaved=json.load(open("/tmp/synthetic-provider-state-resaved.json"))["configuredAccounts"]; assert [item["key"] for item in saved] == [item["key"] for item in reloaded]; assert [item["key"] for item in saved] == [item["key"] for item in resaved]; print("stableKeysConfirmed")'
```

Expected:

- both reload and re-save return `200`
- both duplicate rows keep the same keys across reload and save

## 6. Finish the synthetic link

```bash
FINISH_STATUS=$(curl -sS -o /tmp/synthetic-provider-finish.json -w "%{http_code}" -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections/link-redirect/finish" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"provider\":\"synthetic\",\"state\":\"${STATE}\"}") && test "$FINISH_STATUS" = "200" && CONNECTION_ID=$(STATE="$STATE" python3 -c 'import json, os; data=json.load(open("/tmp/synthetic-provider-finish.json")); assert data["provider"] == "synthetic"; assert data["providerReference"] == os.environ["STATE"]; print(data["id"])') && printf 'connectionId=%s\n' "$CONNECTION_ID"
```

Expected:

- `200`
- response includes a new connection `id`
- response `providerReference` matches `${STATE}`

## 7. Verify the linked synthetic connection exists before sync

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections" -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Expected:

- `200`
- one item matches `${CONNECTION_ID}`
- that item has `provider="synthetic"`, `displayName="Synthetic"`, and `state="active"`

## 8. Trigger one manual sync

Use a fixed window so repeated checks are easy to compare.

```bash
SYNC_STATUS=$(curl -sS -o /tmp/synthetic-provider-sync-trigger.json -w "%{http_code}" -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections/${CONNECTION_ID}/sync" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data '{"reason":"manual","windowStart":"2026-06-01T00:00:00Z","windowEnd":"2026-06-04T00:00:00Z"}') && test "$SYNC_STATUS" = "200" && python3 -c 'import json; data=json.load(open("/tmp/synthetic-provider-sync-trigger.json")); assert data["jobId"]; assert data["jobType"] == "finance.bank_connection_sync"; print("jobId=" + data["jobId"])'
```

Expected:

- `200`
- response includes `jobId`
- response `jobType` is `finance.bank_connection_sync`

## 9. Wait for sync completion

The sync runs asynchronously. Poll the connection until `lastSuccessfulSyncAt` is no longer the zero timestamp.

```bash
for attempt in 1 2 3 4 5 6 7 8 9 10; do curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections" -H "Authorization: Bearer ${ACCESS_TOKEN}" > /tmp/synthetic-provider-connections-after-sync.json && CONNECTION_ID="$CONNECTION_ID" python3 - <<'PY'
import json, os, sys
connection_id = os.environ['CONNECTION_ID']
items = json.load(open('/tmp/synthetic-provider-connections-after-sync.json'))['items']
item = next((x for x in items if x['id'] == connection_id), None)
assert item is not None, 'connection missing during poll'
if item.get('lastSuccessfulSyncAt') and item['lastSuccessfulSyncAt'] != '0001-01-01T00:00:00Z':
    print(item['lastSuccessfulSyncAt'])
    sys.exit(0)
sys.exit(1)
PY
if [ $? -eq 0 ]; then break; fi
sleep 2
done
```

Expected:

- the poll exits successfully within a few attempts
- the connection now shows non-zero `lastSyncStartedAt` and `lastSuccessfulSyncAt`

## 10. Verify linked accounts and provider transactions

List accounts:

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/accounts" -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

List provider transactions:

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/transactions?source=provider&limit=100" -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Optional one-shot assertion:

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/accounts" -H "Authorization: Bearer ${ACCESS_TOKEN}" > /tmp/synthetic-provider-accounts.json && curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/transactions?source=provider&limit=100" -H "Authorization: Bearer ${ACCESS_TOKEN}" > /tmp/synthetic-provider-transactions.json && python3 - <<'PY'
import json
accounts = json.load(open('/tmp/synthetic-provider-accounts.json'))['items']
transactions = json.load(open('/tmp/synthetic-provider-transactions.json'))['items']
assert len(accounts) == 2, accounts
assert {item['name'] for item in accounts} == {'Synthetic Checking'}, accounts
assert len({item['providerAccountId'] for item in accounts}) == 2, accounts
assert all(item['provider'] == 'synthetic' for item in accounts), accounts
account_ids = {item['id'] for item in accounts}
assert len(transactions) > 0, transactions
assert all(item['source'] == 'provider' for item in transactions), transactions
assert all(item['accountId'] in account_ids for item in transactions), transactions
assert all(item.get('providerOriginal') for item in transactions), transactions
print(f'accountCount={len(accounts)}')
print(f'providerTransactions={len(transactions)}')
PY
```

Expected:

- account list returns two linked accounts for this flow
- duplicate configured accounts stay distinct through different `providerAccountId` values
- provider transaction list is non-empty
- provider transactions all point to the linked accounts and include `providerOriginal`

## 11. If anything is wrong, report it

Capture:

- tenant id, state, and connection id
- start response
- initial, saved, reloaded, and re-saved synthetic state responses
- finish response
- sync trigger response
- connection list after sync
- account list response
- transaction list response
