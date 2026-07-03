# Bank Linking Manual E2E

Follow preparation steps in [README.md](./README.md). Get the API token.

## 1. Start link flow

Token link:

```bash
curl -sS -i -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/<tenantId>/connections/link-token" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"provider\":\"monobank\",\"token\":\"${MONOBANK_TOKEN}\"}"
```

Redirect link:

```bash
curl -sS -i -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/<tenantId>/connections/link-redirect/start" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data '{"provider":"pko","callbackUrl":"http://127.0.0.1:5173/#/finance/connections"}'
```

## 2. Finish redirect link

After the provider returns real `code` and `state`:

```bash
curl -sS -i -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/<tenantId>/connections/link-redirect/finish" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data '{"provider":"pko","code":"<realCode>","state":"<realState>"}'
```

## 3. Verify success

```bash
curl -sS -i "http://127.0.0.1:4501/api/v1/finance/tenants/<tenantId>/connections" -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Expected: `200` and the linked connection is listed.
