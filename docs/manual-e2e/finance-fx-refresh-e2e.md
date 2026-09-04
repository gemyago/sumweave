# Finance FX refresh E2E

This is the deterministic API-only gate for
`finance.fx_rates_refresh`. It uses the existing Frankfurter provider pointed
at a local static HTTP fixture, plus the realistic fixture generator's existing
`NewStaticFXProvider` setup. It never calls a public FX endpoint. State and
evidence stay under `tmp/jobs-system-simplification-028-e2e/fx-refresh`.

## 1. Isolated API and local providers

Run from the repository root and use the first `.local-users` entry.

```bash
set -euo pipefail
REPO_ROOT="$PWD"
E2E_ROOT="$REPO_ROOT/tmp/jobs-system-simplification-028-e2e/fx-refresh"
rm -rf "$E2E_ROOT"
mkdir -p "$E2E_ROOT"
# Stop the normal PM2 start-all backend before resetting its shared database.
pm2 stop sumweave-api
docker compose down -v
make postgres-bootstrap
export APP_FINANCE_PROVIDERS_MONOBANK_BASEURL=http://127.0.0.1:4599
export APP_FINANCE_PROVIDERS_FRANKFURTER_BASEURL=http://127.0.0.1:4598

cd "$REPO_ROOT/apps/sumweave"
IFS=: read -r USER PASS < "$REPO_ROOT/.local-users"
go run ./cmd/sumweave --env local user add \
  --username "$USER" --password "$PASS" --if-not-exists

python3 - <<'PY' >"$E2E_ROOT/monobank.log" 2>&1 &
from http.server import BaseHTTPRequestHandler, HTTPServer
class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health": body = b"ok"
        elif self.path == "/personal/client-info": body = b'{"name":"FX bank fixture","accounts":[{"id":"fx-bank-account","type":"black","currencyCode":980,"balance":1}]}'
        elif self.path.startswith("/personal/statement/"): body = b'[]'
        else: self.send_response(404); self.end_headers(); return
        self.send_response(200); self.send_header("Content-Type", "application/json"); self.end_headers(); self.wfile.write(body)
    def log_message(self, *_): pass
HTTPServer(("127.0.0.1", 4599), Handler).serve_forever()
PY
MONOBANK_PID=$!

python3 - <<'PY' >"$E2E_ROOT/fx-provider.log" 2>&1 &
from http.server import BaseHTTPRequestHandler, HTTPServer
class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health": body = b"ok"
        elif self.path.startswith("/v2/rates"): body = b'[{"date":"2026-06-04","base":"EUR","quote":"USD","rate":1.0735}]'
        else: self.send_response(404); self.end_headers(); return
        self.send_response(200); self.send_header("Content-Type", "application/json"); self.end_headers(); self.wfile.write(body)
    def log_message(self, *_): pass
HTTPServer(("127.0.0.1", 4598), Handler).serve_forever()
PY
FX_PID=$!
until curl --fail --silent http://127.0.0.1:4598/health >/dev/null && \
      curl --fail --silent http://127.0.0.1:4599/health >/dev/null; do sleep 1; done

go run ./cmd/sumweave start --env local >"$E2E_ROOT/api.log" 2>&1 &
API_PID=$!
trap 'kill "$API_PID" "$MONOBANK_PID" "$FX_PID" 2>/dev/null || true' EXIT
until curl --fail --silent http://127.0.0.1:4501/health >/dev/null; do sleep 1; done
LOGIN_JSON=$(curl -sS -X POST http://127.0.0.1:4501/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  --data "{\"username\":\"$USER\",\"password\":\"$PASS\"}")
ACCESS_TOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["accessToken"])' <<<"$LOGIN_JSON")
OWNER_USER_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["user"]["id"])' <<<"$LOGIN_JSON")
```

The new `APP_FINANCE_PROVIDERS_FRANKFURTER_BASEURL` config uses the existing
`NewFrankfurterFXProvider` implementation with a local `/v2/rates` response.
The Monobank fixture is needed because the realistic fixture also creates a due
bank schedule, and `enqueue-due` publishes all due finance schedules.

## 2. Seed a deterministic required FX pair

```bash
go run ./cmd/sumweave finance fixtures generate --env local --seed 49 \
  --owner-user-id "$OWNER_USER_ID" --member-user-id "$OWNER_USER_ID" \
  >"$E2E_ROOT/fixtures.json"
TENANT_ID=$(curl -sS http://127.0.0.1:4501/api/v1/finance/tenants \
  -H "Authorization: Bearer $ACCESS_TOKEN" |
  python3 -c 'import json,sys; print(next(x["id"] for x in json.load(sys.stdin)["items"] if x["name"] == "Fixture Tenant"))')
curl -sS http://127.0.0.1:4501/api/v1/finance/fx/diagnostics \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/fx-before.json"
python3 - "$E2E_ROOT/fx-before.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1]))
assert data["storedRatesCount"] > 0
assert data["defaultProvider"] == "frankfurter"
PY
```

The fixture generator's finance module uses deterministic static rates and
persists the EUR/USD pair. This makes the required-pair discovery independent
of the current date or public network.

## 3. Manual success: publish, `404`, worker, terminal state

```bash
curl -sS -X POST http://127.0.0.1:4501/api/v1/finance/fx/sync \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"provider":"frankfurter"}' >"$E2E_ROOT/manual-trigger.json"
MANUAL_JOB_ID=$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["jobType"] == "finance.fx_rates_refresh"; print(d["jobId"])' "$E2E_ROOT/manual-trigger.json")
STATUS=$(curl -sS -o "$E2E_ROOT/manual-job-before.json" -w '%{http_code}' \
  "http://127.0.0.1:4501/api/v1/jobs/$MANUAL_JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN")
test "$STATUS" = 404
go run ./cmd/sumweave jobs worker --once --env local
curl -sS "http://127.0.0.1:4501/api/v1/jobs/$MANUAL_JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/manual-job.json"
curl -sS http://127.0.0.1:4501/api/v1/finance/fx/diagnostics \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/fx-after-manual.json"
python3 - "$E2E_ROOT/manual-job.json" "$E2E_ROOT/fx-after-manual.json" <<'PY'
import json, sys
job, diagnostics = (json.load(open(path)) for path in sys.argv[1:])
assert job["id"] and job["status"] == "succeeded"
assert diagnostics["storedRatesCount"] > 0
PY
```

## 4. Controlled provider business failure

`nbp` is an existing deterministic stub provider. It returns the supported
`fx_provider_not_supported` terminal business failure without making a network
request. Verify that boundary separately:

```bash
curl -sS -X POST http://127.0.0.1:4501/api/v1/finance/fx/sync \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"provider":"nbp"}' >"$E2E_ROOT/failure-trigger.json"
FAILURE_JOB_ID=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["jobId"])' "$E2E_ROOT/failure-trigger.json")
STATUS=$(curl -sS -o "$E2E_ROOT/failure-job-before.json" -w '%{http_code}' \
  "http://127.0.0.1:4501/api/v1/jobs/$FAILURE_JOB_ID" -H "Authorization: Bearer $ACCESS_TOKEN")
test "$STATUS" = 404
go run ./cmd/sumweave jobs worker --once --env local
curl -sS "http://127.0.0.1:4501/api/v1/jobs/$FAILURE_JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/failure-job.json"
python3 - "$E2E_ROOT/failure-job.json" <<'PY'
import json, sys
job = json.load(open(sys.argv[1]))
assert job["status"] == "failed"
assert job["error"]["code"] == "fx_provider_not_supported"
assert "input" not in job and "result" not in job
PY
```

This is a handled business failure and must be acknowledged as a terminal
failed job. Transport, payload, materialization, panic, and terminal-write
errors are not substitutes for this fixture.

## 5. Scheduled FX due state, observation, and repeat

The scheduler command also publishes the due fixture bank command; the local
Monobank fixture above keeps that extra observed workload deterministic.

```bash
# Keep the local Frankfurter fixture above running through enqueue-due and the
# worker pass; this gate must never use a public FX endpoint.
go run ./cmd/sumweave jobs enqueue-due --env local
IFS=$'\t' read -r FX_NEXT_RUN FX_LAST_SCHEDULED SCHEDULED_FX_JOB_ID <<<"$(
  docker compose exec -T -e PGPASSWORD=sumweave_runtime_local postgres \
    psql -p 55432 -U sumweave_runtime -d sumweave_local -At -F $'\t' \
    -c "SELECT next_run_at,last_scheduled_at,last_job_id FROM finance_fx_refresh_schedules WHERE schedule_id = 'finance.fx_rates_daily_refresh'"
)"
test -n "$FX_NEXT_RUN" && test -n "$FX_LAST_SCHEDULED" && test -n "$SCHEDULED_FX_JOB_ID"
STATUS=$(curl -sS -o "$E2E_ROOT/scheduled-fx-before.json" -w '%{http_code}' \
  "http://127.0.0.1:4501/api/v1/jobs/$SCHEDULED_FX_JOB_ID" -H "Authorization: Bearer $ACCESS_TOKEN")
test "$STATUS" = 404
go run ./cmd/sumweave jobs worker --once --env local
curl -sS "http://127.0.0.1:4501/api/v1/jobs/$SCHEDULED_FX_JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/scheduled-fx-job.json"
python3 - "$E2E_ROOT/scheduled-fx-job.json" <<'PY'
import json, sys
assert json.load(open(sys.argv[1]))["status"] == "succeeded"
PY

go run ./cmd/sumweave jobs enqueue-due --env local
IFS=$'\t' read -r FX_REPEAT_NEXT_RUN FX_REPEAT_JOB_ID <<<"$(
  docker compose exec -T -e PGPASSWORD=sumweave_runtime_local postgres \
    psql -p 55432 -U sumweave_runtime -d sumweave_local -At -F $'\t' \
    -c "SELECT next_run_at,last_job_id FROM finance_fx_refresh_schedules WHERE schedule_id = 'finance.fx_rates_daily_refresh'"
)"
test -n "$FX_REPEAT_NEXT_RUN" && test "$FX_REPEAT_JOB_ID" = "$SCHEDULED_FX_JOB_ID"
```

The first scheduled tick must advance `next_run_at`, record
`last_scheduled_at`/`last_job_id`, and leave the job row absent until delivery.
The repeat tick is a no-op while the row is not due: no second message or job
reference is allowed, and no `job_schedules` row is created.

## Cleanup

```bash
kill "$API_PID" "$MONOBANK_PID" "$FX_PID" 2>/dev/null || true
wait "$API_PID" "$MONOBANK_PID" "$FX_PID" 2>/dev/null || true
cd "$REPO_ROOT"
pm2 start ecosystem.config.js
```

This guide owns restoring the normal PM2 backend after its API-only run.
