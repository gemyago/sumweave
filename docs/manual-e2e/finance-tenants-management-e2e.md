# Finance Tenants Management Manual E2E

Follow preparation steps in [README.md](./README.md). Get the API token before starting.

This guide is API-only. Do not use the UI for this check.

## 1. Create a fresh tenant

Use a unique tenant name on each run because archived tenants stay persisted even after they stop appearing in active tenant lists.

```bash
RUN_TAG=$(date +%Y%m%d-%H%M%S) && TENANT_NAME="tenant-e2e-${RUN_TAG}" && CREATE_STATUS=$(curl -sS -o /tmp/finance-tenants-create.json -w "%{http_code}" -X POST "http://127.0.0.1:4501/api/v1/finance/tenants" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"name\":\"${TENANT_NAME}\",\"displayCurrency\":\"USD\"}") && test "$CREATE_STATUS" = "200" && TENANT_ID=$(python3 -c 'import json; print(json.load(open("/tmp/finance-tenants-create.json"))["id"])') && printf 'tenantId=%s\n' "$TENANT_ID"
```

Expected:

- create call returns `200`
- response includes `id`, `name`, `displayCurrency`, `joinedAt`, `createdAt`, and `updatedAt`

## 2. Verify the tenant is listed while active

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants" -H "Authorization: Bearer ${ACCESS_TOKEN}" | TENANT_ID="$TENANT_ID" python3 -c 'import json,os,sys; items=json.load(sys.stdin)["items"]; tenant_id=os.environ["TENANT_ID"]; assert any(item["id"] == tenant_id for item in items), "tenant missing from active list"; print("tenant present in active list")'
```

Expected:

- list call returns `200`
- the new tenant is present in `items`

## 3. Verify the tenant is returned by get-by-id while active

```bash
GET_STATUS=$(curl -sS -o /tmp/finance-tenants-get-before.json -w "%{http_code}" "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}" -H "Authorization: Bearer ${ACCESS_TOKEN}") && test "$GET_STATUS" = "200" && TENANT_ID="$TENANT_ID" python3 -c 'import json,os; item=json.load(open("/tmp/finance-tenants-get-before.json")); assert item["id"] == os.environ["TENANT_ID"], "unexpected tenant returned"; print("tenant returned by get-by-id")'
```

Expected:

- get-by-id call returns `200`
- response tenant `id` matches the created tenant

## 4. Archive the tenant

```bash
ARCHIVE_STATUS=$(curl -sS -o /tmp/finance-tenants-archive.txt -w "%{http_code}" -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/archive" -H "Authorization: Bearer ${ACCESS_TOKEN}") && test "$ARCHIVE_STATUS" = "204" && test ! -s /tmp/finance-tenants-archive.txt && printf 'archive returned %s with empty body\n' "$ARCHIVE_STATUS"
```

Expected:

- archive call returns `204`
- response body is empty

## 5. Verify the archived tenant is gone from the active list

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants" -H "Authorization: Bearer ${ACCESS_TOKEN}" | TENANT_ID="$TENANT_ID" python3 -c 'import json,os,sys; items=json.load(sys.stdin)["items"]; tenant_id=os.environ["TENANT_ID"]; assert all(item["id"] != tenant_id for item in items), "archived tenant still present in active list"; print("archived tenant removed from active list")'
```

Expected:

- list call returns `200`
- the archived tenant no longer appears in `items`

## 6. Verify get-by-id returns not found after archival

```bash
GET_STATUS=$(curl -sS -o /tmp/finance-tenants-get-after.json -w "%{http_code}" "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}" -H "Authorization: Bearer ${ACCESS_TOKEN}") && test "$GET_STATUS" = "404" && printf 'tenant get-by-id returned %s after archival\n' "$GET_STATUS"
```

Expected:

- get-by-id call returns `404`
- the archived tenant is no longer available from the active get-by-id route
