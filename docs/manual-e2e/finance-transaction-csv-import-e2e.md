# Finance transaction CSV import E2E

This is the deterministic API-only gate for `finance.csv_import`. It verifies
the fixed seven-column contract, confirmation publication, lazy job observation,
audit completion, and repeat safety. Use the prepared local PostgreSQL database
with fresh scoped data; do not use PM2 `start-all` for this gate because it can
consume the message before the expected `404`.

## Isolated setup

Run from the repository root and use the first `.local-users` entry.

```bash
set -euo pipefail
REPO_ROOT="$PWD"
E2E_ROOT="$REPO_ROOT/tmp/jobs-system-simplification-028-e2e/transaction-csv"
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
  --data "{\"name\":\"transaction-import-$RUN_ID\",\"displayCurrency\":\"USD\",\"seedDefaults\":false}" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
```

## Preview and confirm

The seven required headers are `Date,Account,Category,Tags,Expense amount,
Income amount,Currency`; `Description` is optional. Dates are strict
`dd.MM.yy`, and amounts may use the documented localized quoted format.

```bash
cat >"$E2E_ROOT/transactions.csv" <<'CSV'
Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description
29.05.26,CSV checking,Groceries,"home, food","8 300,00",,PLN,Monthly groceries
30.05.26,CSV checking,Salary,"work, income",,"12 500,50",PLN,May salary
CSV
python3 - "$E2E_ROOT/transactions.csv" >"$E2E_ROOT/preview-request.json" <<'PY'
import json, sys
print(json.dumps({"fileName": "transactions.csv", "csv": open(sys.argv[1]).read()}))
PY
curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/imports/preview" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data @"$E2E_ROOT/preview-request.json" >"$E2E_ROOT/preview.json"
python3 - "$E2E_ROOT/preview.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1]))
assert data["importableCount"] == 2
assert data["rejectedRows"] == [] and data["duplicateRows"] == []
assert set(data["headers"]) == {"Date", "Account", "Category", "Tags", "Expense amount", "Income amount", "Currency", "Description"}
assert data["accountOptions"][0]["selected"]
print("preview-ok")
PY
IMPORT_ID=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["importId"])' "$E2E_ROOT/preview.json")

# Confirmation has no request body: the preview owns the import payload.
curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/imports/$IMPORT_ID/confirm" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/confirm.json"
JOB_ID=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["jobId"])' "$E2E_ROOT/confirm.json")
JOB_TYPE=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["jobType"])' "$E2E_ROOT/confirm.json")
test "$JOB_TYPE" = finance.csv_import
JOB_STATUS=$(curl -sS -o "$E2E_ROOT/job-before-delivery.json" -w '%{http_code}' \
  "http://127.0.0.1:4501/api/v1/jobs/$JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN")
test "$JOB_STATUS" = 404
```

The returned ID is the appdispatch message ID and future observed job ID. The
pre-worker `404` is required: publication does not create a job row.

## Worker, terminal state, and repeat safety

```bash
go run ./cmd/sumweave jobs worker --once --env local
curl -sS "http://127.0.0.1:4501/api/v1/jobs/$JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/job.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/imports/$IMPORT_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/audit.json"
python3 - "$E2E_ROOT/job.json" "$E2E_ROOT/audit.json" <<'PY'
import json, sys
job, audit = (json.load(open(path)) for path in sys.argv[1:])
assert job["id"] == audit["jobId"]
assert job["status"] == "succeeded"
assert audit["status"] == "completed"
assert "input" not in job and "result" not in job
PY

# The domain confirmation is idempotent: repeat confirmation returns the same
# reference with 200 and does not publish a second command or duplicate rows.
REPEAT_STATUS=$(curl -sS -o "$E2E_ROOT/repeat-confirm.json" -w '%{http_code}' -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/imports/$IMPORT_ID/confirm" \
  -H "Authorization: Bearer $ACCESS_TOKEN")
test "$REPEAT_STATUS" = 200
python3 - "$E2E_ROOT/confirm.json" "$E2E_ROOT/repeat-confirm.json" <<'PY'
import json, sys
first, repeat = (json.load(open(path)) for path in sys.argv[1:])
assert repeat == first
PY
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/imports/$IMPORT_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/audit-repeat.json"
python3 - "$E2E_ROOT/audit.json" "$E2E_ROOT/audit-repeat.json" <<'PY'
import json, sys
first, repeat = (json.load(open(path)) for path in sys.argv[1:])
assert repeat["jobId"] == first["jobId"]
assert repeat["importedCount"] == first["importedCount"]
PY
```

Inspect the audit row outcomes and verify the imported PLN transactions, account,
category, tags, and optional descriptions. Job detail is lifecycle/requester/
worker/attempt metadata plus sanitized error fields only.

## Controlled validation fixture

Preview a second CSV containing an invalid date, both amount columns populated,
and an unsupported currency. Expect a `200` preview with deterministic row
number/reason diagnostics and `importableCount=0`; do not confirm it. This is the
supported business-validation branch and must not be reported as a transport
failure.

```bash
cat >"$E2E_ROOT/transactions-invalid.csv" <<'CSV'
Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description
31.02.26,CSV checking,Invalid,,1.00,2.00,PLN,invalid date and amounts
01.06.26,CSV checking,Invalid,,1.00,,GBP,unsupported currency
CSV
python3 - "$E2E_ROOT/transactions-invalid.csv" >"$E2E_ROOT/invalid-preview-request.json" <<'PY'
import json, sys
print(json.dumps({"fileName": "transactions-invalid.csv", "csv": open(sys.argv[1]).read()}))
PY
curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/imports/preview" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data @"$E2E_ROOT/invalid-preview-request.json" >"$E2E_ROOT/invalid-preview.json"
python3 - "$E2E_ROOT/invalid-preview.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1]))
assert data["importableCount"] == 0
assert len(data["rejectedRows"]) == 2
PY
```

## Narrow-screen UI spot check

After the API gate, the same tenant/import may be opened at `390×844` in
`#/finance/imports` to verify readable preview diagnostics, audit outcomes, and
the job link. This is optional for the API-only gate and must use the normal UI
manual-E2E setup, not the isolated API process.

## Cleanup

```bash
kill "$API_PID" 2>/dev/null || true
wait "$API_PID" 2>/dev/null || true
cd "$REPO_ROOT"
pm2 start ecosystem.config.js
```

This guide owns restoring the normal PM2 backend after its API-only run.
