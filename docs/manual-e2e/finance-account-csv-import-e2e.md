# Finance account CSV import E2E

This is the deterministic API-only gate for `finance.account_import`. It uses
the prepared local PostgreSQL database with fresh scoped data. Do not use the
normal PM2 `start-all` process for this gate: it would consume the message before
the expected pre-materialization `404` check.

## Isolated setup

Run from the repository root. The first `.local-users` entry is used for the
isolated database.

```bash
set -euo pipefail
REPO_ROOT="$PWD"
E2E_ROOT="$REPO_ROOT/tmp/jobs-system-simplification-028-e2e/account-csv"
rm -rf "$E2E_ROOT"
mkdir -p "$E2E_ROOT"
RUN_ID="$(date +%s)"
# Stop the normal PM2 start-all backend before resetting its shared database.
pm2 stop sumweave-api
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
  --data "{\"name\":\"account-import-$RUN_ID\",\"displayCurrency\":\"USD\",\"seedDefaults\":false}" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
```

## Preview, publish, and verify the pending window

```bash
cat >"$E2E_ROOT/accounts.csv" <<'CSV'
name,currency,kind
Imported checking,PLN,manual
Imported savings,EUR,manual
CSV
python3 - "$E2E_ROOT/accounts.csv" >"$E2E_ROOT/preview-request.json" <<'PY'
import json, sys
print(json.dumps({"fileName": "accounts.csv", "csv": open(sys.argv[1]).read()}))
PY
curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/account-imports/preview" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data @"$E2E_ROOT/preview-request.json" >"$E2E_ROOT/preview.json"
python3 - "$E2E_ROOT/preview.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1]))
assert data["rejectedRows"] == []
assert set(data["wouldCreateAccounts"]) == {"Imported checking", "Imported savings"}
assert data["headers"] == ["name", "currency", "kind"]
print(data["importId"])
PY
IMPORT_ID=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["importId"])' "$E2E_ROOT/preview.json")

curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/account-imports/$IMPORT_ID/confirm" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/confirm.json"
JOB_ID=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["jobId"])' "$E2E_ROOT/confirm.json")
JOB_TYPE=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["jobType"])' "$E2E_ROOT/confirm.json")
test "$JOB_TYPE" = finance.account_import
JOB_STATUS=$(curl -sS -o "$E2E_ROOT/job-before-delivery.json" -w '%{http_code}' \
  "http://127.0.0.1:4501/api/v1/jobs/$JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN")
test "$JOB_STATUS" = 404
```

The confirmation response is the appdispatch message ID. No job row exists
until delivery, so `404` here is required and is not an error in this gate.

## Worker, terminal state, and repeat safety

```bash
go run ./cmd/sumweave jobs worker --once --env local
curl -sS "http://127.0.0.1:4501/api/v1/jobs/$JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/job.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/account-imports/$IMPORT_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/audit.json"
python3 - "$E2E_ROOT/job.json" "$E2E_ROOT/audit.json" <<'PY'
import json, sys
job, audit = (json.load(open(path)) for path in sys.argv[1:])
assert job["id"] == audit["jobId"]
assert job["status"] == "succeeded"
assert audit["status"] == "completed"
assert audit["jobId"] == job.get("id", audit["jobId"])
assert "input" not in job and "result" not in job
PY

# Reconfirming the same import is idempotent: it returns the same reference with
# 200 and creates no second dispatch or duplicate account rows.
REPEAT_STATUS=$(curl -sS -o "$E2E_ROOT/repeat-confirm.json" -w '%{http_code}' -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/account-imports/$IMPORT_ID/confirm" \
  -H "Authorization: Bearer $ACCESS_TOKEN")
test "$REPEAT_STATUS" = 200
python3 - "$E2E_ROOT/confirm.json" "$E2E_ROOT/repeat-confirm.json" <<'PY'
import json, sys
first, repeat = (json.load(open(path)) for path in sys.argv[1:])
assert repeat == first
PY
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/accounts" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/accounts.json"
python3 - "$E2E_ROOT/accounts.json" <<'PY'
import json, sys
items = json.load(open(sys.argv[1]))["items"]
assert len([item for item in items if item["name"] in {"Imported checking", "Imported savings"}]) == 2
PY
```

The job detail may contain lifecycle, requester, worker, attempt, and sanitized
error fields only. It must not contain stored input, progress, or result data.

## Controlled invalid fixture

For the supported business-validation branch, preview a second CSV containing
one row with a missing `currency`. Expect a `200` preview with the deterministic
missing-fields diagnostic and no accounts to create; do not confirm it. This
exercises a rejected-row outcome without making a transport failure look like a
business failure.

```bash
cat >"$E2E_ROOT/accounts-invalid.csv" <<'CSV'
name,currency,kind
Invalid account,,manual
CSV
python3 - "$E2E_ROOT/accounts-invalid.csv" >"$E2E_ROOT/invalid-preview-request.json" <<'PY'
import json, sys
print(json.dumps({"fileName": "accounts-invalid.csv", "csv": open(sys.argv[1]).read()}))
PY
curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/account-imports/preview" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data @"$E2E_ROOT/invalid-preview-request.json" >"$E2E_ROOT/invalid-preview.json"
python3 - "$E2E_ROOT/invalid-preview.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1]))
assert data["headers"] == ["name", "currency", "kind"]
assert data["rejectedRows"] == [{"rowNumber": 2, "reason": "account row is missing required fields"}]
assert data["wouldCreateAccounts"] == []
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
