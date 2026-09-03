# Finance scheduled bank-sync lifecycle E2E

This is the deterministic API-only gate for a scheduled
`finance.bank_connection_sync`. It uses the realistic fixture generator, a
local Monobank HTTP fixture, and the local static Frankfurter-compatible FX
fixture. All state and evidence live under
`tmp/jobs-system-simplification-028-e2e/scheduled-bank`; no public provider
network is used.

## 1. Isolated API, bank fixture, and static FX fixture

Run from the repository root. The first `.local-users` entry is used.

```bash
set -euo pipefail
REPO_ROOT="$PWD"
E2E_ROOT="$REPO_ROOT/tmp/jobs-system-simplification-028-e2e/scheduled-bank"
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
        if self.path == "/health":
            body = b"ok"
        elif self.path == "/personal/client-info":
            body = b'{"name":"Fixture Connection","accounts":[{"id":"fixture-external","type":"black","currencyCode":980,"balance":101}]}'
        elif self.path.startswith("/personal/statement/"):
            body = b'[{"id":"fixture-statement-1","time":1710000000,"description":"fixture","amount":-101,"currencyCode":980,"balance":0}]'
        else:
            self.send_response(404); self.end_headers(); return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers(); self.wfile.write(body)
    def log_message(self, *_): pass

HTTPServer(("127.0.0.1", 4599), Handler).serve_forever()
PY
MONOBANK_PID=$!

python3 - <<'PY' >"$E2E_ROOT/fx-provider.log" 2>&1 &
from http.server import BaseHTTPRequestHandler, HTTPServer

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health":
            body = b"ok"
        elif self.path.startswith("/v2/rates"):
            body = b'[{"date":"2026-06-04","base":"EUR","quote":"USD","rate":1.0735}]'
        else:
            self.send_response(404); self.end_headers(); return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers(); self.wfile.write(body)
    def log_message(self, *_): pass

HTTPServer(("127.0.0.1", 4598), Handler).serve_forever()
PY
FX_PID=$!
until curl --fail --silent http://127.0.0.1:4599/health >/dev/null && \
      curl --fail --silent http://127.0.0.1:4598/health >/dev/null; do sleep 1; done

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

The `APP_FINANCE_PROVIDERS_FRANKFURTER_BASEURL` setting uses the existing
Frankfurter provider implementation with a repository-local HTTP endpoint. It
is the required static/no-network setup for both the scheduler and worker.

## 2. Generate the due bank fixture and inspect initial state

```bash
go run ./cmd/sumweave finance fixtures generate --env local --seed 49 \
  --owner-user-id "$OWNER_USER_ID" --member-user-id "$OWNER_USER_ID" \
  >"$E2E_ROOT/fixtures.json"
TENANT_ID=$(curl -sS http://127.0.0.1:4501/api/v1/finance/tenants \
  -H "Authorization: Bearer $ACCESS_TOKEN" |
  python3 -c 'import json,sys; print(next(x["id"] for x in json.load(sys.stdin)["items"] if x["name"] == "Fixture Tenant"))')
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/connections-before.json"
python3 - "$E2E_ROOT/connections-before.json" <<'PY'
import json, sys
item = json.load(open(sys.argv[1]))["items"][0]
assert item["state"] == "active" and item["schedule"]["enabled"] is True
assert item["schedule"]["nextRunAt"]
assert not item["schedule"]["lastScheduledAt"]
assert not item["schedule"]["lastJobId"]
PY
```

The fixture generator itself uses `NewStaticFXProvider` and writes deterministic
seed rates. The worker refresh below additionally proves the configured local
HTTP provider path.

## 3. Publish due bank and FX commands, then prove both pending IDs

```bash
# Both local fixtures above must still be running before enqueue-due. This
# command must never fall back to a public FX endpoint.
go run ./cmd/sumweave jobs enqueue-due --env local
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/connections-after-dispatch.json"
BANK_JOB_ID=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["items"][0]["schedule"]["lastJobId"])' "$E2E_ROOT/connections-after-dispatch.json")
IFS=$'\t' read -r FX_NEXT_RUN_BEFORE FX_LAST_SCHEDULED_BEFORE FX_JOB_ID <<<"$(
  docker compose exec -T -e PGPASSWORD=sumweave_runtime_local postgres \
    psql -p 55432 -U sumweave_runtime -d sumweave_local -At -F $'\t' \
    -c "SELECT next_run_at,last_scheduled_at,last_job_id FROM finance_fx_refresh_schedules WHERE schedule_id = 'finance.fx_rates_daily_refresh' AND provider = 'frankfurter'"
)"
test -n "$FX_NEXT_RUN_BEFORE" && test -n "$FX_LAST_SCHEDULED_BEFORE" && test -n "$FX_JOB_ID"
for JOB_ID in "$BANK_JOB_ID" "$FX_JOB_ID"; do
  STATUS=$(curl -sS -o "$E2E_ROOT/job-$JOB_ID-before-delivery.json" -w '%{http_code}' \
    "http://127.0.0.1:4501/api/v1/jobs/$JOB_ID" -H "Authorization: Bearer $ACCESS_TOKEN")
  test "$STATUS" = 404
done
```

`enqueue-due` publishes semantic finance commands and advances finance-owned
bank/FX due state; it does not execute either workload or create job rows. The
two `404` responses are the required API-only publication window.

## 4. Run one bounded worker pass and verify terminal outcomes

```bash
go run ./cmd/sumweave jobs worker --once --env local
for JOB_ID in "$BANK_JOB_ID" "$FX_JOB_ID"; do
  curl -sS "http://127.0.0.1:4501/api/v1/jobs/$JOB_ID" \
    -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/job-$JOB_ID.json"
done
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/connections-after.json"
curl -sS http://127.0.0.1:4501/api/v1/finance/fx/diagnostics \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/fx-diagnostics.json"
python3 - "$E2E_ROOT/connections-after.json" "$E2E_ROOT/fx-diagnostics.json" "$E2E_ROOT/job-$BANK_JOB_ID.json" "$E2E_ROOT/job-$FX_JOB_ID.json" <<'PY'
import json, sys
connections, diagnostics, bank_job, fx_job = (json.load(open(path)) for path in sys.argv[1:])
connection = connections["items"][0]
assert connection["lastSyncJobId"] == connection["schedule"]["lastJobId"]
assert connection["lastSuccessfulSyncAt"] and connection["schedule"]["lastCompletedAt"]
assert bank_job["id"] == connection["lastSyncJobId"] and bank_job["status"] == "succeeded"
assert fx_job["id"] and fx_job["status"] == "succeeded"
assert diagnostics["storedRatesCount"] > 0
assert any(item["name"] == "frankfurter" and item["ready"] for item in diagnostics["providers"])
PY
```

The bank and FX jobs must both be terminal. With the supplied fixtures they
should be `succeeded`; a sanitized `failed` result is only acceptable when the
fixture or local provider was intentionally changed to a supported terminal
business failure. Provider/network, payload, materialization, or terminal
state-write failures are transport failures and are not a passing business
failure result.

## 5. Verify due-state advancement and repeat/idempotency

```bash
IFS=$'\t' read -r FX_NEXT_RUN_AFTER FX_LAST_SCHEDULED_AFTER FX_LAST_JOB_AFTER <<<"$(
  docker compose exec -T -e PGPASSWORD=sumweave_runtime_local postgres \
    psql -p 55432 -U sumweave_runtime -d sumweave_local -At -F $'\t' \
    -c "SELECT next_run_at,last_scheduled_at,last_job_id FROM finance_fx_refresh_schedules WHERE schedule_id = 'finance.fx_rates_daily_refresh'"
)"
test "$FX_NEXT_RUN_AFTER" != "$FX_NEXT_RUN_BEFORE"
test "$FX_LAST_SCHEDULED_AFTER" = "$FX_LAST_SCHEDULED_BEFORE"
test "$FX_LAST_JOB_AFTER" = "$FX_JOB_ID"

go run ./cmd/sumweave jobs enqueue-due --env local
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/connections" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/connections-after-repeat.json"
python3 - "$E2E_ROOT/connections-after.json" "$E2E_ROOT/connections-after-repeat.json" <<'PY'
import json, sys
first, repeat = (json.load(open(path)) for path in sys.argv[1:])
assert first["items"][0]["schedule"]["lastJobId"] == repeat["items"][0]["schedule"]["lastJobId"]
PY
FX_REPEAT_JOB_ID=$(docker compose exec -T -e PGPASSWORD=sumweave_runtime_local postgres \
  psql -p 55432 -U sumweave_runtime -d sumweave_local -At \
  -c "SELECT last_job_id FROM finance_fx_refresh_schedules WHERE schedule_id = 'finance.fx_rates_daily_refresh'")
test "$FX_REPEAT_JOB_ID" = "$FX_JOB_ID"
```

The second scheduler tick is a no-op because both finance-owned schedules are
no longer due. It must not publish a second occurrence, change either future
reference, or create a `job_schedules` row.

## Cleanup

```bash
kill "$API_PID" "$MONOBANK_PID" "$FX_PID" 2>/dev/null || true
wait "$API_PID" "$MONOBANK_PID" "$FX_PID" 2>/dev/null || true
cd "$REPO_ROOT"
pm2 start ecosystem.config.js
```

This guide owns restoring the normal PM2 backend after its API-only run.
