# Integration Layer Typical-Scenario Manual E2E

Follow [README.md](./README.md) first. This is the Phase 1 completion smoke for
personal access tokens and the direct `swmd` client. It uses real local
PostgreSQL, the split API/worker process model, the browser UI, and a locally
built client. It does not use a second integration API or a mock HTTP server.

## 1. Prepare the local system and fixture data

Run from the repository root. Keep all evidence in the temporary run directory;
issued token values stay only in shell variables and must not be written to JSON
artifacts or screenshots.

```bash
set -euo pipefail
RUN_ID="integration-$(date +%s)"
E2E_ROOT="$PWD/tmp/integration-layer-e2e-$RUN_ID"
SWMD_CONFIG="$E2E_ROOT/swmd.json"
mkdir -p "$E2E_ROOT"
make postgres-bootstrap
pm2 delete backend || true
pm2 delete ui || true
pm2 start ecosystem.config.js
until curl --fail --silent http://127.0.0.1:4501/health >/dev/null; do sleep 1; done
until curl --fail --silent -I http://127.0.0.1:5173/ >/dev/null; do sleep 1; done
(cd apps/sumweave && go build -o "$E2E_ROOT/swmd" ./cmd/swmd)
IFS=: read -r USER PASS < .local-users
SESSION_TOKEN=$(curl -fsS -X POST http://127.0.0.1:4501/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  --data "{\"username\":\"$USER\",\"password\":\"$PASS\"}" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["accessToken"])')
```

Prepare a fresh session-owned tenant with a manual account and transactions, a
synthetic connection, and current account and transaction provider snapshots.
Use the session-only fixture instructions in
[synthetic-provider-flow-e2e.md](./synthetic-provider-flow-e2e.md), especially
the classification fixtures and synthetic connection setup. Retain these safe
identifiers in shell variables: `TENANT_ID`, `ACCOUNT_ID`, `TRANSACTION_ID`,
`CONNECTION_ID`, `ACCOUNT_SNAPSHOT_ID`, and `TRANSACTION_SNAPSHOT_ID`.

Stop only the normal PM2 worker before publishing the lazy-job checks; leave API
and UI running:

```bash
pm2 stop worker
```

## 2. Create, list, rotate, and revoke credentials in the UI

1. Sign in at `http://127.0.0.1:5173/#/login`, then open
   `#/admin/access-tokens`.
2. At `1280x900`, create a uniquely named read-only token. Copy its displayed
   one-time value into `READ_TOKEN`; dismiss the result and reload. Confirm the
   list is newest-first metadata only—no secret or digest is present.
3. Rotate that active token with a confirmation. Confirm the old value is
   immediately unusable, copy the one-time replacement, and revoke it with its
   confirmation. Confirm its next request is rejected.
4. Create a uniquely named read-write token and copy its one-time value into
   `WRITE_TOKEN` for the write checks.
5. At `390x844`, repeat the screen check: creation, list metadata, rotate/revoke
   confirmations, one-time result, explicit copy, and dismiss are keyboard
   usable with no clipped controls or horizontal overflow. Capture desktop and
   narrow screenshots in `$E2E_ROOT` when evidence is needed.

The page is self-service, not an administrator surface. Dismiss, navigation, or
reload must remove an issued secret from the page. Do not place either token in
screenshots, shell history, or retained JSON.

## 3. Configure the built client and verify the read allowlist

Persist configuration only through the exact stdin contract. `auth status` must
redact the token; `swmd auth configure --token` must be rejected because no
secret-bearing flag exists.

```bash
printf '%s\n' "$READ_TOKEN" | "$E2E_ROOT/swmd" --config "$SWMD_CONFIG" auth configure \
  --base-url http://127.0.0.1:4501 --token-stdin
"$E2E_ROOT/swmd" --config "$SWMD_CONFIG" auth status >"$E2E_ROOT/auth-status.json"
"$E2E_ROOT/swmd" --config "$SWMD_CONFIG" tenant list >"$E2E_ROOT/tenants.json"
"$E2E_ROOT/swmd" --config "$SWMD_CONFIG" account list --tenant "$TENANT_ID" >"$E2E_ROOT/accounts.json"
"$E2E_ROOT/swmd" --config "$SWMD_CONFIG" transaction list --tenant "$TENANT_ID" --limit 1 --offset 0 >"$E2E_ROOT/transactions-page-1.json"
"$E2E_ROOT/swmd" --config "$SWMD_CONFIG" transaction list --tenant "$TENANT_ID" --limit 1 --offset 1 >"$E2E_ROOT/transactions-page-2.json"
"$E2E_ROOT/swmd" --config "$SWMD_CONFIG" connection list --tenant "$TENANT_ID" >"$E2E_ROOT/connections.json"
"$E2E_ROOT/swmd" --config "$SWMD_CONFIG" account provider-data-list --tenant "$TENANT_ID" --account "$ACCOUNT_ID" >"$E2E_ROOT/account-snapshots.json"
"$E2E_ROOT/swmd" --config "$SWMD_CONFIG" account provider-data-get --tenant "$TENANT_ID" --account "$ACCOUNT_ID" --snapshot "$ACCOUNT_SNAPSHOT_ID" >"$E2E_ROOT/account-snapshot.json"
"$E2E_ROOT/swmd" --config "$SWMD_CONFIG" transaction provider-data-list --tenant "$TENANT_ID" --transaction "$TRANSACTION_ID" >"$E2E_ROOT/transaction-snapshots.json"
"$E2E_ROOT/swmd" --config "$SWMD_CONFIG" transaction provider-data-get --tenant "$TENANT_ID" --transaction "$TRANSACTION_ID" --snapshot "$TRANSACTION_SNAPSHOT_ID" >"$E2E_ROOT/transaction-snapshot.json"
```

Confirm `auth-status.json` has `"apiToken":"redacted"`; all successful client
outputs are one complete JSON value; the two offset pages differ when the tenant
has at least two transactions; snapshot metadata omits documents; and snapshot
details are sanitized. The token must access only its owner's current tenant
membership.

Use this read-only token against all three trigger routes and representative
session-only runtime, token-management, and finance-mutation routes. Each must
return `403 insufficient_permission`; none may publish work.

## 4. Publish all three writes and prove lazy job materialization

Replace the temporary config with the read-write one-time token. Submit the
three approved triggers with offset-preserving timestamp ranges and record only
the returned IDs.

```bash
rm -f "$SWMD_CONFIG"
printf '%s\n' "$WRITE_TOKEN" | "$E2E_ROOT/swmd" --config "$SWMD_CONFIG" auth configure \
  --base-url http://127.0.0.1:4501 --token-stdin
SYNC_ID=$("$E2E_ROOT/swmd" --config "$SWMD_CONFIG" connection sync --tenant "$TENANT_ID" \
  --connection "$CONNECTION_ID" --reason manual --window-start 2026-06-01T00:00:00Z \
  --window-end 2026-06-04T00:00:00Z --idempotency-key "$RUN_ID-sync" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["jobId"])')
CLASSIFY_ID=$("$E2E_ROOT/swmd" --config "$SWMD_CONFIG" classification run --tenant "$TENANT_ID" \
  --range-start 2026-06-01T00:00:00-04:00 --range-end-exclusive 2026-06-04T00:00:00-04:00 \
  --idempotency-key "$RUN_ID-classify" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["jobId"])')
MATCH_ID=$("$E2E_ROOT/swmd" --config "$SWMD_CONFIG" transfer match --tenant "$TENANT_ID" \
  --range-start 2026-06-01T00:00:00-04:00 --range-end-exclusive 2026-06-04T00:00:00-04:00 \
  --idempotency-key "$RUN_ID-match" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["jobId"])')
for ID in "$SYNC_ID" "$CLASSIFY_ID" "$MATCH_ID"; do
  test "$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:4501/api/v1/jobs/$ID" -H "Authorization: Bearer $WRITE_TOKEN")" = 404
done
```

Each trigger must return `202`, derive the `integration` requester, and produce
no eager job row. The final loop proves the expected initial `404` before first
delivery. Run a bounded real worker from the backend module, then let `swmd job
wait` poll each known ID through its bounded materialization grace period to a
terminal status.

```bash
(cd apps/sumweave && go run ./cmd/sumweave jobs worker --once --env local)
for ID in "$SYNC_ID" "$CLASSIFY_ID" "$MATCH_ID"; do
  "$E2E_ROOT/swmd" --config "$SWMD_CONFIG" job wait --job "$ID" --interval 1s --timeout 1m >"$E2E_ROOT/job-$ID.json"
done
pm2 restart worker
```

The synthetic sync, classification, and matching fixture should succeed. If a
deliberately configured provider failure is tested instead, record its sanitized
failed job and the nonzero `swmd job wait` exit separately.

## 5. Prove retry safety, requester isolation, and revocation

1. Repeat one supported trigger with exactly the same body and idempotency key.
   Confirm its `jobId` equals the original. Reuse that key with a changed body;
   confirm `409 idempotency_conflict` and no second job.
2. While the PM2 worker is stopped, publish the same supported trigger with the
   browser session. Its requester source is `operator`; a `WRITE_TOKEN` job get
   must return `404`. Create a second local user with an owned fixture/job and
   confirm the first token also receives `404` for that job and an unknown ID.
   If a due scheduled job exists, confirm it too is `404` to the token. The
   owning browser session sees only its own operator and integration jobs.
3. Rotate the read-write token in the browser and prove the old value returns
   `401` immediately. With the replacement, submit one accepted trigger, then
   revoke the replacement in the browser. Its next request must return `401`.
   Run `jobs worker --once` and verify that the job accepted before revocation
   reaches a terminal state even though the revoked token cannot read it.

## 6. Inspect and clean up

1. Inspect `$E2E_ROOT`, browser console/network diagnostics, and
   `pm2 logs backend --lines 200`. Confirm no bearer value, digest, provider
   credential, raw provider HTTP body, or raw idempotency key appears.
2. Revoke every E2E token in the browser. Clear the temporary client config,
   remove E2E artifacts, restart the ordinary worker if needed, and leave PM2
   healthy.

```bash
"$E2E_ROOT/swmd" --config "$SWMD_CONFIG" auth clear || true
rm -rf "$E2E_ROOT"
pm2 status
```

Record scenario names, IDs, statuses, response codes, screenshot paths, and
cleanup result in the handoff. Never record a complete token or other secret.
