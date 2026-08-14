# Final Review

Whole-change independent review passed after remediation and the final Mock ASPSP
acceptance passed. The change is approved and ready for user review.

## Whole-change review — 2026-08-14

**Approved.** Review 016 findings were remediated and verification in review 018
found no remaining whole-change issues. The final Mock ASPSP E2E in review 019
then passed, including PKO authorization/linking, durable sync, typed source
snapshots, secret-safe API/UI disclosure, and authenticated legacy-route 404s.

## Operator action required after upgrade

GORM auto-migrate creates `finance_provider_snapshots` but does not drop tables that are no longer registered. For an upgraded persistent database, the operator MUST first confirm that the new application version is running successfully and that `finance_provider_snapshots` exists. After confirming that rollback to a legacy build is not required, the operator MUST run these statements against the Sumweave application database:

```sql
DROP TABLE IF EXISTS finance_provider_evidence;
DROP TABLE IF EXISTS finance_raw_payloads;
```

The operator MUST confirm that both retired tables are absent after executing the statements. No row copying or backup for these tables is required by this change. Fresh databases and recreated local/test databases require no manual drop.

## Whole-change review remediation — 2026-08-14

- Pending redirect-start documents are now decoded, credential-sanitized, and
  re-encoded before pending-start persistence. Invalid JSON fails the redirect
  start before the pending store or current-snapshot store is touched.
- Regression coverage proves nested credential-like sentinels are absent from
  the saved pending document, no pending redirect start invokes current snapshot
  storage, and invalid documents produce no pending write.
- Removed the stale `ProviderRawPayload` public-DTO claim from the active
  bank-connection v2 boundary note.
- Focused finance checks, finance module lint/test, repository
  `make affected-lint-test`, and strict OpenSpec validation passed.

## Final Mock ASPSP acceptance — 2026-08-14

**Passed.** After the user completed the interactive CAPTCHA and email sign-in
in the headed browser, the final run authorized `PKO E2E Sample` through Mock
ASPSP, created active PKO connection provider reference
`79ded516-f201-4e9c-a9c5-609253e373ca`, and completed durable job
`01a00135-9c5c-7cca-99ef-4007b2168174` as
`finance.bank_connection_sync` / `succeeded` with one attempt.

The selected tenant was `PKO typed snapshots E2E 20260814`. Sumweave exposed
one linked EUR account and three provider transactions. Account source data
revealed distinct current `account` and `account_balance` snapshots, and the
transaction source data revealed a complete typed transaction item. API detail
responses were present and credential-safe. Authenticated account and
transaction `/evidence` list routes both returned HTTP 404 with no redirects.
Final Playwright console and PM2 API error checks were clean. No product code
was changed.

Two inherited pending starts initially targeted an HTTPS localhost callback
while the documented standard HTTP PM2 services were running. The API was
recreated with the local-only callback override
`APP_FINANCE_PROVIDERS_ENABLEBANKING_CALLBACKBASEURL=http://localhost:4501`,
then the flow was rerun successfully; this did not require a code change.

Detailed commands, IDs, safe artifacts, and limitations are recorded in
`tmp/crew-manager/adopt-typed-provider-source-snapshots-019-crew-p2-high-notes.md`.
