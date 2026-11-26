---
type: Runbook
title: "Bank Linking Manual E2E"
description: "Manual verification procedure for bank linking e2e."
sources:
  - id: bank-poc-guidance
    resource: "https://github.com/gemyago/sumweave/tree/99bac4b630a02983551c0775de946f8749a9d3b1/apps/sumweave/doc/financial-poc"
    title: "Operator constraints extracted from the retired bank POC notes"
  - id: monobank-poc-command
    resource: "https://github.com/gemyago/sumweave/blob/99bac4b630a02983551c0775de946f8749a9d3b1/apps/sumweave/cmd/sumweave/finance_poc_monobank_transactions.go"
    title: "Current bounded Monobank diagnostic CLI behavior"
  - id: callback-contract
    resource: "https://github.com/gemyago/sumweave/blob/99bac4b630a02983551c0775de946f8749a9d3b1/apps/sumweave-ui/src/pages/FinanceConnections.test.ts"
    title: "Current callback success and retry tests"
  - id: bank-linking-e2e
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/docs/manual-e2e/bank-linking-e2e.md"
    title: "docs/manual-e2e/bank-linking-e2e.md"
  - id: documentation-snapshot
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/docs/manual-e2e/bank-linking-e2e.md"
    title: "Reviewed documentation checkpoint before this format migration"
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T12:07:03+00:00" }
---

# Bank Linking Manual E2E

Follow preparation steps in [README.md](./README.md). Get the API token.[^documentation-snapshot][^bank-linking-e2e]

## Provider setup and callback recovery

Keep provider tokens, private keys, pending-auth/session files, and exports out
of tracked files and logs. Human bank login and strong customer authentication
(SCA) are required for live bank authorization; sign-in or CAPTCHA may also block
unattended sandbox work. Production bank access may require approval in the bank
and provider setup. Use the [Mock ASPSP UI guide](enable-banking-mock-aspsp-ui-e2e.md)
for the configured sandbox and [local HTTPS](../local-https.md) when a trusted
local callback certificate is needed.[^bank-poc-guidance]

The app callback is `/enable-banking/callback`. Set
`APP_FINANCE_PROVIDERS_ENABLEBANKING_CALLBACKBASEURL` when the registered callback
origin differs from the backend host. Check the configured ASPSP before blaming
the callback: `APP_FINANCE_PROVIDERS_ENABLEBANKING_ASPSPNAME` must identify an entry
exposed by that sandbox app. `Mock ASPSP` is a configured example, not a guarantee
about the provider's live inventory.[^bank-poc-guidance]

The browser returns to `/?code=...&state=...#/finance/connections`. Finish using
server-side pending state and the returned code/state. Clear the query only after
success. On a transient finish failure, preserve it and reopen the same route to
retry; do not start another link just to recover the existing pending attempt.[^callback-contract]

## Optional provider diagnostics

The retained `finance-poc` commands call providers directly and do not create a
product connection. Run them from `apps/sumweave` only for deliberate diagnostics.
They use their own environment variables: Enable Banking requires
`ENABLE_BANKING_APP_ID` and `ENABLE_BANKING_PRIVATE_KEY_PATH`, with optional
`ENABLE_BANKING_BASE_URL`; Monobank uses `MONOBANK_TOKEN` and optional
`MONOBANK_BASE_URL`. The application ID is the JWT `kid`; issuer and audience are
fixed as `enablebanking.com` and `api.enablebanking.com`.[^bank-poc-guidance]

```sh
go run ./cmd/sumweave finance-poc enable-banking aspsps --country PL --json
go run ./cmd/sumweave finance-poc enable-banking start-auth --help
go run ./cmd/sumweave finance-poc enable-banking finish-session --help
go run ./cmd/sumweave finance-poc monobank accounts --json
go run ./cmd/sumweave finance-poc monobank transactions --help
```

For the manual Enable Banking diagnostic path, `start-auth` writes a pending
`--auth-file`; after human authorization, `finish-session --auth-file ... --code
... --state ...` writes the `--session-file`. `accounts` and `transactions` read
that session file. The same-machine `connect` alternative takes
`--callback-listen-addr`; `--callback-cert-file` and `--callback-key-file` must be
provided together and cover the callback host. Omitting both generates an
in-memory self-signed certificate that a bank or browser may reject. A reachable
registered redirect URL can be used with the manual path instead.[^bank-poc-guidance]

The Monobank diagnostic transactions command accepts `--account`, `--from`,
`--to`, and optional `--out`; its default account is `0`. It splits ranges at
**31 days plus 1 hour** and waits **61 seconds** between chunks by default.
These are CLI limits, separate from the product sync's 30-calendar-day windows.
Keep live ranges small and avoid repeated polling against the real provider;
inspect offline or fake-provider results first. Store temporary diagnostics under
repository `tmp/`; existing POC export/session ignore rules remain in place.[^monobank-poc-command][^bank-poc-guidance]

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

[^bank-poc-guidance]: Retired Enable Banking/PKO, sandbox, and Monobank operator notes at the pinned revision; retained setup and recovery constraints, without claiming current live inventory or a completed run.
[^monobank-poc-command]: Current command flags, account default, range chunking, and inter-chunk delay constants.
[^callback-contract]: Registered UI tests preserve code/state after transient failure and clear them only after a successful finish.
