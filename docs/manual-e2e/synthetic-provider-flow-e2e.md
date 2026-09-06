# Synthetic provider bank-sync and classification E2E

This is the deterministic API-only gate for a manual
`finance.bank_connection_sync`. It uses the in-process synthetic connector and
the prepared local PostgreSQL database. Every non-database artifact stays under
the repository `tmp/` directory. The synthetic fixture has no supported
business-failure response.
The terminal success and pending-state key-preservation checks below are the
required assertions for this flow; the synthetic transaction generator does not
promise manual-sync transaction-count idempotency. The fixed synthetic window
and `Synthetic debit` rule make automatic and explicit classification
assertions deterministic without changing provider behavior.

## Isolated setup

Run from the repository root and use the first `.local-users` entry.

```bash
set -euo pipefail
REPO_ROOT="$PWD"
E2E_ROOT="$REPO_ROOT/tmp/classification-phase0-033-e2e/synthetic-bank"
rm -rf "$E2E_ROOT"
mkdir -p "$E2E_ROOT"
RUN_ID="$(date +%s)"
# Stop the normal PM2 API and worker before resetting their shared database.
pm2 stop backend
docker compose down -v
make postgres-bootstrap

cd "$REPO_ROOT/apps/sumweave"
IFS=: read -r USER PASS < "$REPO_ROOT/.local-users"
go run ./cmd/sumweave --env local user add \
  --username "$USER" --password "$PASS" --if-not-exists
go run ./cmd/sumweave start --env local >"$E2E_ROOT/api.log" 2>&1 &
API_PID=$!
trap 'kill "$API_PID" 2>/dev/null || true' EXIT
until curl --fail --silent http://127.0.0.1:4501/health >/dev/null; do sleep 1; done
LOGIN_JSON=$(curl -sS -X POST http://127.0.0.1:4501/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  --data "{\"username\":\"$USER\",\"password\":\"$PASS\"}")
ACCESS_TOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["accessToken"])' <<<"$LOGIN_JSON")
TENANT_ID=$(curl -sS -X POST http://127.0.0.1:4501/api/v1/finance/tenants \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"name\":\"synthetic-bank-$RUN_ID\",\"displayCurrency\":\"USD\",\"seedDefaults\":false}" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
```

## Classification fixtures

Create the categories, a manual account, a pre-categorized transaction, and the
automatic rule before the synthetic sync. The synthetic connector's local
fixture uses a zero offset and debit value for the fixed window below, so every
provider transaction description begins with `Synthetic debit`.

```bash
AUTO_CATEGORY_ID=$(curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/categories" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"name":"Synthetic automatic","kind":"expense"}' |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
PROTECTED_CATEGORY_ID=$(curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/categories" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"name":"Synthetic protected","kind":"expense"}' |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
MANUAL_ACCOUNT_ID=$(curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/accounts" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"name":"Classification fixtures","currency":"USD","kind":"manual"}' |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

# This matching description must remain protected after automatic and explicit
# delivery: both classifiers only fill transactions with no category.
curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"accountId\":\"$MANUAL_ACCOUNT_ID\",\"source\":\"manual\",\"status\":\"booked\",\"kind\":\"expense\",\"amountMinor\":-101,\"currency\":\"USD\",\"description\":\"Synthetic debit protected\",\"effectiveAt\":\"2026-06-01T12:00:00-04:00\",\"categoryId\":\"$PROTECTED_CATEGORY_ID\",\"tagIds\":[]}" \
  >"$E2E_ROOT/protected-transaction.json"

curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/classification-rules" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"matchType\":\"contains\",\"condition\":\"Synthetic debit\",\"categoryId\":\"$AUTO_CATEGORY_ID\"}" \
  >"$E2E_ROOT/automatic-rule.json"
python3 - "$E2E_ROOT/protected-transaction.json" "$E2E_ROOT/automatic-rule.json" <<'PY'
import json, sys
protected, rule = (json.load(open(path)) for path in sys.argv[1:])
assert protected["categoryId"] and rule["id"]
PY
```

## Link synthetic accounts

```bash
START_STATUS=$(curl -sS -o "$E2E_ROOT/link-start.json" -w '%{http_code}' -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections/link-redirect/start" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"provider":"synthetic","callbackUrl":"http://127.0.0.1:5173/#/finance/connections"}')
test "$START_STATUS" = 200
STATE=$(python3 - "$E2E_ROOT/link-start.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1]))
assert data["provider"] == "synthetic"
assert "#/finance/connections/synthetic?state=" in data["authorizationUrl"]
assert data["state"]
print(data["state"])
PY
)

curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections/synthetic-link-states/state/$STATE" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/state-initial.json"
python3 - "$E2E_ROOT/state-initial.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1]))
assert data["configuredAccounts"] == [] and data["canFinish"] is False
PY

SAVE_BODY='{"configuredAccounts":[{"name":"Synthetic Checking","currency":"USD"},{"name":"Synthetic Savings","currency":"EUR"}]}'
curl -sS -X PUT \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections/synthetic-link-states/state/$STATE" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "$SAVE_BODY" >"$E2E_ROOT/state-saved.json"
python3 - "$E2E_ROOT/state-saved.json" <<'PY'
import json, sys
items = json.load(open(sys.argv[1]))["configuredAccounts"]
assert len(items) == 2 and items[0]["key"] != items[1]["key"]
PY

# Reload and save the same payload, including the returned keys. Keys are the
# pending rows' identities; preserving them proves a resave updates those rows
# instead of allocating new configured accounts.
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections/synthetic-link-states/state/$STATE" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/state-reloaded.json"
SAVE_BODY_KEYED=$(python3 - "$E2E_ROOT/state-reloaded.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1]))
print(json.dumps({"configuredAccounts": data["configuredAccounts"]}))
PY
)
curl -sS -X PUT \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections/synthetic-link-states/state/$STATE" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "$SAVE_BODY_KEYED" >"$E2E_ROOT/state-resaved.json"
python3 - "$E2E_ROOT/state-saved.json" "$E2E_ROOT/state-reloaded.json" "$E2E_ROOT/state-resaved.json" <<'PY'
import json, sys
keys = [[item["key"] for item in json.load(open(path))["configuredAccounts"]] for path in sys.argv[1:]]
assert keys[0] == keys[1] == keys[2]
PY

curl -sS -X POST http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections/link-redirect/finish \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"provider\":\"synthetic\",\"state\":\"$STATE\"}" >"$E2E_ROOT/link-finish.json"
CONNECTION_ID=$(STATE="$STATE" python3 - "$E2E_ROOT/link-finish.json" <<'PY'
import json, os, sys
data = json.load(open(sys.argv[1]))
assert data["provider"] == "synthetic" and data["providerReference"] == os.environ["STATE"]
print(data["id"])
PY
)
```

Verify the connection list has one active synthetic connection with the new
ID, then publish a fixed-window sync while the API-only process is still the
only consumer:

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/connections-before-sync.json"
CONNECTION_ID="$CONNECTION_ID" python3 - "$E2E_ROOT/connections-before-sync.json" <<'PY'
import json, os, sys
items = json.load(open(sys.argv[1]))["items"]
item = next(item for item in items if item["id"] == os.environ["CONNECTION_ID"])
assert item["provider"] == "synthetic" and item["state"] == "active"
PY
curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections/$CONNECTION_ID/sync" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"reason":"manual","windowStart":"2026-06-01T00:00:00Z","windowEnd":"2026-06-04T00:00:00Z"}' \
  >"$E2E_ROOT/sync-trigger.json"
JOB_ID=$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["jobType"] == "finance.bank_connection_sync"; print(d["jobId"])' "$E2E_ROOT/sync-trigger.json")
JOB_STATUS=$(curl -sS -o "$E2E_ROOT/job-before-delivery.json" -w '%{http_code}' \
  "http://127.0.0.1:4501/api/v1/jobs/$JOB_ID" -H "Authorization: Bearer $ACCESS_TOKEN")
test "$JOB_STATUS" = 404
```

The `404` proves that the API published an appdispatch command without running
bank work inline or fabricating a queued job row.

## Worker, terminal state, bank results, and automatic classification

```bash
go run ./cmd/sumweave jobs worker --once --env local
# The bank job emits ordinary automatic-classification work. A second bounded
# drain makes that separate delivery explicit even if the first drain consumed it.
go run ./cmd/sumweave jobs worker --once --env local
curl -sS "http://127.0.0.1:4501/api/v1/jobs/$JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/job.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/connections-after-sync.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/accounts" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/accounts.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions?source=provider&limit=100" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/transactions.json"
CONNECTION_ID="$CONNECTION_ID" JOB_ID="$JOB_ID" AUTO_CATEGORY_ID="$AUTO_CATEGORY_ID" python3 \
  "$E2E_ROOT/job.json" "$E2E_ROOT/connections-after-sync.json" "$E2E_ROOT/accounts.json" "$E2E_ROOT/transactions.json" <<'PY'
import json, os, sys
job, connections, accounts, transactions = (json.load(open(path)) for path in sys.argv[1:])
connection = next(item for item in connections["items"] if item["id"] == os.environ["CONNECTION_ID"])
assert job["id"] == os.environ["JOB_ID"]
assert job["status"] == "succeeded"
assert connection["lastSuccessfulSyncAt"]
assert len([item for item in accounts["items"] if item["provider"] == "synthetic"]) == 2
assert transactions["items"] and all(item["source"] == "provider" for item in transactions["items"])
assert all(item["description"].startswith("Synthetic debit") for item in transactions["items"])
assert all(item["categoryId"] == os.environ["AUTO_CATEGORY_ID"] for item in transactions["items"])
PY
```

The required results are: terminal observed job, non-null
`lastSuccessfulSyncAt`, two distinct synthetic provider accounts, and non-empty
provider transactions. The rule-triggered automatic delivery must classify each
synthetic provider transaction with `Synthetic automatic`.

## Explicit classification and category-preservation checks

Create one matching uncategorized manual transaction, add its rule, then submit
the inclusive visible dates as an offset-preserving, half-open range. The `404`
before delivery is valid only for this initiating classification request.

```bash
curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"accountId\":\"$MANUAL_ACCOUNT_ID\",\"source\":\"manual\",\"status\":\"booked\",\"kind\":\"expense\",\"amountMinor\":-202,\"currency\":\"USD\",\"description\":\"Explicit classification fixture\",\"effectiveAt\":\"2026-06-02T12:00:00-04:00\",\"tagIds\":[]}" \
  >"$E2E_ROOT/explicit-transaction.json"
EXPLICIT_TRANSACTION_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/explicit-transaction.json")
curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/classification-rules" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"matchType\":\"contains\",\"condition\":\"Explicit classification fixture\",\"categoryId\":\"$AUTO_CATEGORY_ID\"}" \
  >"$E2E_ROOT/explicit-rule.json"

curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions/classify" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"rangeStart":"2026-06-01T00:00:00-04:00","rangeEndExclusive":"2026-06-04T00:00:00-04:00"}' \
  >"$E2E_ROOT/classify-trigger.json"
CLASSIFY_JOB_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["jobId"])' <"$E2E_ROOT/classify-trigger.json")
CLASSIFY_BEFORE_DELIVERY=$(curl -sS -o "$E2E_ROOT/classify-before-delivery.json" -w '%{http_code}' \
  "http://127.0.0.1:4501/api/v1/jobs/$CLASSIFY_JOB_ID" -H "Authorization: Bearer $ACCESS_TOKEN")
test "$CLASSIFY_BEFORE_DELIVERY" = 404

go run ./cmd/sumweave jobs worker --once --env local
curl -sS "http://127.0.0.1:4501/api/v1/jobs/$CLASSIFY_JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/classify-job.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions?limit=100" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/classified-transactions.json"
EXPLICIT_TRANSACTION_ID="$EXPLICIT_TRANSACTION_ID" AUTO_CATEGORY_ID="$AUTO_CATEGORY_ID" PROTECTED_CATEGORY_ID="$PROTECTED_CATEGORY_ID" python3 \
  "$E2E_ROOT/classify-job.json" "$E2E_ROOT/classified-transactions.json" <<'PY'
import json, os, sys
job, transactions = (json.load(open(path)) for path in sys.argv[1:])
items = transactions["items"]
explicit = next(item for item in items if item["id"] == os.environ["EXPLICIT_TRANSACTION_ID"])
protected = next(item for item in items if item["description"] == "Synthetic debit protected")
assert job["status"] == "succeeded"
assert explicit["categoryId"] == os.environ["AUTO_CATEGORY_ID"]
assert protected["categoryId"] == os.environ["PROTECTED_CATEGORY_ID"]
PY
```

Repeat the fixed bank sync and its automatic-classification delivery. The
pre-categorized fixture must remain protected across repeated sync and delivery.

```bash
curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections/$CONNECTION_ID/sync" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"reason":"manual","windowStart":"2026-06-01T00:00:00Z","windowEnd":"2026-06-04T00:00:00Z"}' \
  >"$E2E_ROOT/repeat-sync-trigger.json"
REPEAT_SYNC_JOB_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["jobId"])' <"$E2E_ROOT/repeat-sync-trigger.json")
go run ./cmd/sumweave jobs worker --once --env local
go run ./cmd/sumweave jobs worker --once --env local
curl -sS "http://127.0.0.1:4501/api/v1/jobs/$REPEAT_SYNC_JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/repeat-sync-job.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions?limit=100" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/repeated-transactions.json"
PROTECTED_CATEGORY_ID="$PROTECTED_CATEGORY_ID" python3 - "$E2E_ROOT/repeat-sync-job.json" "$E2E_ROOT/repeated-transactions.json" <<'PY'
import json, os, sys
job, transactions = (json.load(open(path)) for path in sys.argv[1:])
assert job["status"] == "succeeded"
items = transactions["items"]
protected = next(item for item in items if item["description"] == "Synthetic debit protected")
assert protected["categoryId"] == os.environ["PROTECTED_CATEGORY_ID"]
PY
```

## Cleanup

```bash
kill "$API_PID" 2>/dev/null || true
wait "$API_PID" 2>/dev/null || true
cd "$REPO_ROOT"
pm2 start ecosystem.config.js
```

This guide owns restoring the normal PM2 backend after its API-only run.
