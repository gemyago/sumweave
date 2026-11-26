---
type: Runbook
title: "Bank Linking Manual E2E"
description: "Manual verification procedure for bank linking e2e."
sources:
  - id: bank-linking-e2e
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/docs/manual-e2e/bank-linking-e2e.md"
    title: "docs/manual-e2e/bank-linking-e2e.md"
  - id: documentation-snapshot
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/docs/manual-e2e/bank-linking-e2e.md"
    title: "Reviewed documentation checkpoint before this format migration"
---

# Bank Linking Manual E2E

Follow preparation steps in [README.md](./README.md). Get the API token.[^documentation-snapshot][^bank-linking-e2e]

## 1. Start link flow

Token link:[^documentation-snapshot]

```bash
curl -sS -i -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/<tenantId>/connections/link-token" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"provider\":\"monobank\",\"token\":\"${MONOBANK_TOKEN}\"}"
```

Redirect link:

```bash
curl -sS -i -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/<tenantId>/connections/link-redirect/start" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data '{"provider":"pko","callbackUrl":"http://127.0.0.1:5173/#/finance/connections"}'
```

## 2. Finish redirect link

After the provider returns real `code` and `state`:[^documentation-snapshot]

```bash
curl -sS -i -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/<tenantId>/connections/link-redirect/finish" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data '{"provider":"pko","code":"<realCode>","state":"<realState>"}'
```

## 3. Verify success

```bash
curl -sS -i "http://127.0.0.1:4501/api/v1/finance/tenants/<tenantId>/connections" -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Expected: `200` and the linked connection is listed.[^documentation-snapshot]

[^documentation-snapshot]: Reviewed documentation checkpoint, preserved as the source of this section; format and navigation changes do not establish execution results.

[^bank-linking-e2e]: docs/manual-e2e/bank-linking-e2e.md.
