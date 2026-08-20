# Synthetic provider bank-sync E2E

This is the deterministic API-only gate for a manual
`finance.bank_connection_sync`. It uses the in-process synthetic connector and
an isolated SQLite database. Every artifact stays under the repository `tmp/`
directory. The synthetic fixture has no supported business-failure response.
The terminal success and pending-state key-preservation checks below are the
required assertions for this flow; the synthetic transaction generator does not
promise manual-sync transaction-count idempotency.

## Isolated setup

Run from the repository root and use the first `.local-users` entry.

```bash
set -euo pipefail
REPO_ROOT="$PWD"
E2E_ROOT="$REPO_ROOT/tmp/jobs-system-simplification-028-e2e/synthetic-bank"
rm -rf "$E2E_ROOT"
mkdir -p "$E2E_ROOT"
export APP_DATADIR="$E2E_ROOT/data"
export APP_APPLICATION_DATABASE_DSN="$E2E_ROOT/application.db"

cd "$REPO_ROOT/apps/sumweave"
go run ./cmd/sumweave db-migrate --env local
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
  --data '{"name":"synthetic-bank-022","displayCurrency":"USD","seedDefaults":false}' |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
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

## Worker, terminal state, and bank results

```bash
go run ./cmd/sumweave jobs worker --once --env local
curl -sS "http://127.0.0.1:4501/api/v1/jobs/$JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/job.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/connections-after-sync.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/accounts" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/accounts.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions?source=provider&limit=100" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/transactions.json"
CONNECTION_ID="$CONNECTION_ID" JOB_ID="$JOB_ID" python3 \
  "$E2E_ROOT/job.json" "$E2E_ROOT/connections-after-sync.json" "$E2E_ROOT/accounts.json" "$E2E_ROOT/transactions.json" <<'PY'
import json, os, sys
job, connections, accounts, transactions = (json.load(open(path)) for path in sys.argv[1:])
connection = next(item for item in connections["items"] if item["id"] == os.environ["CONNECTION_ID"])
assert job["id"] == os.environ["JOB_ID"]
assert job["status"] == "succeeded"
assert connection["lastSuccessfulSyncAt"]
assert len([item for item in accounts["items"] if item["provider"] == "synthetic"]) == 2
assert transactions["items"] and all(item["source"] == "provider" for item in transactions["items"])
PY
```

The required results are: terminal observed job, non-null
`lastSuccessfulSyncAt`, two distinct synthetic provider accounts, and non-empty
provider transactions.

## Cleanup

```bash
kill "$API_PID" 2>/dev/null || true
wait "$API_PID" 2>/dev/null || true
```
