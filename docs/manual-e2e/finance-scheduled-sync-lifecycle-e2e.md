# Finance Scheduled Sync Lifecycle Manual E2E

Follow preparation steps in [README.md](./README.md) first.

This flow runs backend CLI commands from `apps/signal-foundry`. It uses a fresh isolated local DB so
`jobs worker --once` stays bounded. Keep the local Monobank mock running for the
fixture, worker, and UI verification steps.

## 1. Start an isolated API-only backend from the app root

```bash
REPO_ROOT="$PWD"
E2E_ROOT="$REPO_ROOT/tmp/timestamp-nullable-cleanup-052-e2e"
rm -rf "$E2E_ROOT"
mkdir -p "$E2E_ROOT"
export APP_DATADIR="$E2E_ROOT/data"
export APP_DATALAYER_DATABASE_DSN="$E2E_ROOT/data-layer.db"
export APP_FINANCE_FIXTURES_DATABASE_DSN="$APP_DATALAYER_DATABASE_DSN"

pm2 stop signal-foundry-api
cd apps/signal-foundry
go run ./cmd/signal-foundry db-migrate --env local
go run ./cmd/signal-foundry start --env local >"$E2E_ROOT/api.log" 2>&1 &
API_PID=$!
until curl --fail --silent http://127.0.0.1:4501/health >/dev/null; do sleep 1; done
```

## 2. Provision or reuse the local user against the isolated DB, then log in

Use the first `.local-users` entry. If you need to create or rotate it, do that
now with the isolated API environment still exported, then log in and take
`user.id` from the auth JSON. Do not scrape `user add` stdout; it can include
DEBUG noise.

```bash
IFS=: read -r USER PASS < "$REPO_ROOT/.local-users"
LOGIN_JSON=$(curl -sS -X POST "http://127.0.0.1:4501/api/v1/auth/login" -H "Content-Type: application/json" --data "{\"username\":\"${USER}\",\"password\":\"${PASS}\"}")
ACCESS_TOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["accessToken"])' <<<"$LOGIN_JSON")
OWNER_USER_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["user"]["id"])' <<<"$LOGIN_JSON")
```

## 3. Start the local Monobank mock and wait for readiness

```bash
python3 - <<'PY' &
from http.server import BaseHTTPRequestHandler, HTTPServer

class H(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok")
            return
        if self.path == "/personal/client-info":
            body = b'{"name":"Fixture Connection","accounts":[{"id":"fixture-external","type":"black","currencyCode":980,"balance":101}]}'
        elif self.path.startswith("/personal/statement/"):
            body = b'[{"id":"fixture-statement-1","time":1710000000,"description":"fixture","amount":-101,"currencyCode":980,"balance":0}]'
        else:
            self.send_response(404)
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        pass

HTTPServer(("127.0.0.1", 4599), H).serve_forever()
PY
MOCK_PID=$!
until curl --fail --silent http://127.0.0.1:4599/health >/dev/null; do sleep 1; done
export APP_FINANCE_PROVIDERS_MONOBANK_BASEURL=http://127.0.0.1:4599
```

## 4. Generate fixtures and verify the pre-dispatch state

```bash
go run ./cmd/signal-foundry finance fixtures generate --env local --seed 49 --owner-user-id "$OWNER_USER_ID" --member-user-id "$OWNER_USER_ID" >"$E2E_ROOT/fixtures.json"
TENANT_ID=$(curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants" -H "Authorization: Bearer ${ACCESS_TOKEN}" | python3 -c 'import json,sys; items=json.load(sys.stdin)["items"]; print(next(item["id"] for item in items if item["name"] == "Fixture Tenant"))')
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections" -H "Authorization: Bearer ${ACCESS_TOKEN}" >"$E2E_ROOT/connections-before.json"
```

Expected before dispatch:

- one active `Fixture Connection`
- `schedule.enabled` is `true`
- `schedule.nextRunAt` is present, non-empty, and not year-one/zero
- `schedule.lastScheduledAt`, `schedule.lastStartedAt`, and `schedule.lastCompletedAt` are omitted or `null`

## 5. Enqueue due work and run one bounded worker pass

```bash
go run ./cmd/signal-foundry jobs enqueue-due --env local
APP_FINANCE_PROVIDERS_MONOBANK_BASEURL=http://127.0.0.1:4599 \
  go run ./cmd/signal-foundry jobs worker --once --env local
```

`--once` stays bounded here because the DB is fresh and isolated.

## 6. Verify the public connection, jobs, and job-detail responses

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections" -H "Authorization: Bearer ${ACCESS_TOKEN}" >"$E2E_ROOT/connections-after.json"
curl -sS "http://127.0.0.1:4501/api/v1/jobs?limit=100" -H "Authorization: Bearer ${ACCESS_TOKEN}" >"$E2E_ROOT/jobs.json"
JOB_ID=$(python3 -c 'import json,sys; data=json.load(open(sys.argv[1])); item=next(x for x in data["items"] if x["displayName"] == "Fixture Connection"); print(item["schedule"]["lastJobId"])' "$E2E_ROOT/connections-after.json")
curl -sS "http://127.0.0.1:4501/api/v1/jobs/${JOB_ID}" -H "Authorization: Bearer ${ACCESS_TOKEN}" >"$E2E_ROOT/job.json"
```

Expected after the worker pass:

- `schedule.lastScheduledAt`, `schedule.lastStartedAt`, and `schedule.lastCompletedAt` are present, non-zero, and preserve their offset
- `lastSuccessfulSyncAt` is present on the connection top level, non-zero, and preserves its offset
- `schedule.nextRunAt` is present and later than `schedule.lastScheduledAt`
- `schedule.lastJobId == connection.lastSyncJobId`
- `lastSyncError` is absent
- jobs list/detail return `200`
- the Finance job detail returns `jobType == finance.bank_connection_sync`
- `startedAt`, `completedAt`, and `lastAttemptAt` are present
- `input` and `result` are omitted, not fabricated as year-one timestamps

## 7. Verify the job-detail UI and browser checks

- On desktop, open the connection page, click `Open last sync job`, and confirm
  the job-detail page renders the Finance sync job instead of the runtime-error
  alert.
- The job-detail page should show the actual job type (`finance.bank_connection_sync`)
  plus requester, worker, attempts, and lifecycle timestamps.
- Repeat at 375px mobile width. Confirm the connection card and job-detail page
  do not overflow or truncate the lifecycle values.
- In the browser console and network panels for both pages, confirm there are
  no errors, warnings, or failed requests, and every page/request returns 200.

## 8. Restore PM2

```bash
API_LISTENER_PID=$(lsof -ti tcp:4501)
[ -n "$API_LISTENER_PID" ] && kill "$API_LISTENER_PID"
kill "$API_PID" "$MOCK_PID"
wait "$API_PID" 2>/dev/null || true
until ! lsof -ti tcp:4501 >/dev/null; do sleep 1; done
cd "$REPO_ROOT"
pm2 restart signal-foundry-api
```
