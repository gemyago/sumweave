# Finance transfer matching E2E

This is the deterministic verification runbook for Phase 0 transfer matching.
It is deliberately API-first: it creates isolated PostgreSQL data through the
supported Finance API, proves the API-only job-observation window, then uses a
bounded worker. Use the synthetic provider only once to emit a real committed
bank-sync-window event; it is not the source of the matching fixtures.

Do not run this destructive guide while another developer needs the local
database. Keep every captured response, command log, and screenshot under the
run directory below.

## 1. Isolated API-only setup

Follow the shared setup in [README.md](./README.md), then run from the
repository root. This keeps the normal worker stopped until the guide says to
run it. The first `.local-users` entry is the default local identity.

```bash
set -euo pipefail
REPO_ROOT="$PWD"
E2E_ROOT="$REPO_ROOT/tmp/transfer-matching-phase0-e2e"
rm -rf "$E2E_ROOT"
mkdir -p "$E2E_ROOT"
RUN_ID="$(date +%s)"

pm2 stop backend
docker compose down -v
make postgres-bootstrap

cd "$REPO_ROOT/apps/sumweave"
IFS=: read -r USER PASS < "$REPO_ROOT/.local-users"
go run ./cmd/sumweave --env local user add --username "$USER" --password "$PASS" --if-not-exists

# Build and launch the server binary directly so API_PID is the listening
# process, rather than the `go run` parent. The binary is isolated run evidence.
go build -o "$E2E_ROOT/sumweave-api" ./cmd/sumweave
"$E2E_ROOT/sumweave-api" start --env local >"$E2E_ROOT/api.log" 2>&1 &
API_PID=$!
stop_api_only() {
  if kill -0 "$API_PID" 2>/dev/null; then
    kill "$API_PID"
  fi
  wait "$API_PID" 2>/dev/null || true
  for _ in $(seq 1 30); do
    if ! lsof -nP -iTCP:4501 -sTCP:LISTEN >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.1
  done
  lsof -nP -iTCP:4501 -sTCP:LISTEN >&2 || true
  return 1
}
restore_pm2() {
  cd "$REPO_ROOT"
  pm2 start ecosystem.config.js
  pm2 jlist | python3 -c 'import json,sys; apps={app["name"]:app["pm2_env"]["status"] for app in json.load(sys.stdin)}; assert {name:apps.get(name) for name in ("api","worker","ui")} == {"api":"online","worker":"online","ui":"online"}, apps'
  curl --fail --silent http://127.0.0.1:4501/health >/dev/null
}
cleanup_api_only() {
  local original_status="$1" cleanup_status=0
  stop_api_only || cleanup_status=1
  restore_pm2 || cleanup_status=1
  if [ "$original_status" -ne 0 ] || [ "$cleanup_status" -ne 0 ]; then
    return 1
  fi
}
trap 'cleanup_api_only "$?"' EXIT
until curl --fail --silent http://127.0.0.1:4501/health >/dev/null; do sleep 1; done

LOGIN_JSON=$(curl -sS -X POST http://127.0.0.1:4501/api/v1/auth/login \
  -H 'Content-Type: application/json' --data "{\"username\":\"$USER\",\"password\":\"$PASS\"}")
ACCESS_TOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["accessToken"])' <<<"$LOGIN_JSON")
TENANT_ID=$(curl -sS -X POST http://127.0.0.1:4501/api/v1/finance/tenants \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"name\":\"transfer-matching-$RUN_ID\",\"displayCurrency\":\"USD\",\"seedDefaults\":false}" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
```

Create four visible manual accounts and retain their IDs in the evidence
directory. The third account supports ambiguity and correction checks; the EUR
account supplies the cross-currency rejection fixture.

```bash
create_account() {
  curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/accounts" \
    -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
    --data "{\"name\":\"$1\",\"currency\":\"$2\",\"kind\":\"manual\"}"
}
create_account 'Matching checking' USD >"$E2E_ROOT/account-a.json"
create_account 'Matching savings' USD >"$E2E_ROOT/account-b.json"
create_account 'Matching ambiguity' USD >"$E2E_ROOT/account-c.json"
create_account 'Matching EUR' EUR >"$E2E_ROOT/account-eur.json"
ACCOUNT_A=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/account-a.json")
ACCOUNT_B=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/account-b.json")
ACCOUNT_C=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/account-c.json")
ACCOUNT_EUR=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/account-eur.json")
```

## 2. Create deterministic API fixtures

The API has no fixture endpoint for a transfer-matching run. Use normal manual
ledger records. All times below include explicit offsets so the submitted range
and the 72-hour elapsed boundary are inspectable without assuming UTC.

Create one category and tag, then these booked manual records:

- `unique-out` and `unique-in`: equal and opposite `500` USD on different
  accounts exactly 72 elapsed hours apart; `unique-out` has the category/tag.
- `outside-in`: the unique partner is outside the narrow explicit range but
  inside 72 hours of an in-range `outside-out` leg.
- `ambiguous-out`, `ambiguous-in-a`, and `ambiguous-in-b`: equal values that
  leave each candidate ambiguous.
- `beyond-out` and `beyond-in`: equal and opposite values more than 72 hours
  apart, which must remain regular.
- `classified`: an uncategorized non-transfer record for classification-event
  independence. Add a matching classification rule for its description.
- Completion-gate rows below exercise account and tenant isolation, eligibility
  rejection, uniqueness, range extension, paging/filter independence, and both
  classification orderings. Give every fixture its exact description shown here;
  the assertions use those saved response files rather than list ordering.

The following helper uses only the supported transaction-create contract. The
June window is intentional: it is also the fixed synthetic window used by the
real-event section later in this guide.

```bash
CATEGORY_ID=$(curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/categories" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"name":"Transfer preserved","kind":"expense"}' |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
TAG_ID=$(curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/tags" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"name":"Transfer retained"}' |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

create_transaction() {
  local account_id="$1" amount_minor="$2" description="$3" effective_at="$4" category_id="$5" tag_ids="$6"
  local body
  body=$(python3 - "$account_id" "$amount_minor" "$description" "$effective_at" "$category_id" "$tag_ids" <<'PY'
import json, sys
account_id, amount_minor, description, effective_at, category_id, tag_ids = sys.argv[1:]
body = {
  "accountId": account_id, "source": "manual", "status": "booked", "kind": "regular",
  "amountMinor": int(amount_minor), "currency": "USD", "description": description,
  "effectiveAt": effective_at, "tagIds": json.loads(tag_ids),
}
if category_id:
  body["categoryId"] = category_id
print(json.dumps(body))
PY
)
  curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions" \
    -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' --data "$body"
}

create_fixture() {
  local tenant_id="$1" account_id="$2" amount_minor="$3" description="$4"
  local effective_at="$5" currency="$6" status="$7" kind="$8" category_id="$9"
  local tag_ids="${10}"
  local body
  body=$(python3 - "$account_id" "$amount_minor" "$description" "$effective_at" "$currency" "$status" "$kind" "$category_id" "$tag_ids" <<'PY'
import json, sys
account_id, amount_minor, description, effective_at, currency, status, kind, category_id, tag_ids = sys.argv[1:]
body = {
  "accountId": account_id, "source": "manual", "status": status, "kind": kind,
  "amountMinor": int(amount_minor), "currency": currency, "description": description,
  "effectiveAt": effective_at, "tagIds": json.loads(tag_ids),
}
if category_id:
  body["categoryId"] = category_id
print(json.dumps(body))
PY
)
  curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$tenant_id/transactions" \
    -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' --data "$body"
}

create_manual_link() {
  curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions/transfer-links" \
    -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
    --data "{\"firstTransactionId\":\"$1\",\"secondTransactionId\":\"$2\"}"
}

# Exactly 72 elapsed hours: 2026-06-01T14:00Z through 2026-06-04T14:00Z.
create_transaction "$ACCOUNT_A" -500 'unique-out' '2026-06-01T10:00:00-04:00' "$CATEGORY_ID" "[\"$TAG_ID\"]" >"$E2E_ROOT/unique-out.json"
create_transaction "$ACCOUNT_B" 500 'unique-in' '2026-06-04T10:00:00-04:00' '' '[]' >"$E2E_ROOT/unique-in.json"
# The partner is outside the explicit [June 1, June 3) range but inside 72 hours.
create_transaction "$ACCOUNT_A" -600 'outside-out' '2026-06-02T12:00:00-04:00' '' '[]' >"$E2E_ROOT/outside-out.json"
create_transaction "$ACCOUNT_B" 600 'outside-in' '2026-06-03T12:00:00-04:00' '' '[]' >"$E2E_ROOT/outside-in.json"
# One debit and two eligible credits are intentionally ambiguous.
create_transaction "$ACCOUNT_A" -700 'ambiguous-out' '2026-06-02T08:00:00-04:00' '' '[]' >"$E2E_ROOT/ambiguous-out.json"
create_transaction "$ACCOUNT_B" 700 'ambiguous-in-a' '2026-06-02T09:00:00-04:00' '' '[]' >"$E2E_ROOT/ambiguous-in-a.json"
create_transaction "$ACCOUNT_C" 700 'ambiguous-in-b' '2026-06-02T10:00:00-04:00' '' '[]' >"$E2E_ROOT/ambiguous-in-b.json"
# This pair is 73 elapsed hours apart and must remain regular.
create_transaction "$ACCOUNT_A" -800 'beyond-out' '2026-06-01T10:00:00-04:00' '' '[]' >"$E2E_ROOT/beyond-out.json"
create_transaction "$ACCOUNT_B" 800 'beyond-in' '2026-06-04T11:00:00-04:00' '' '[]' >"$E2E_ROOT/beyond-in.json"
create_transaction "$ACCOUNT_C" -321 'classified' '2026-06-02T12:00:00-04:00' '' '[]' >"$E2E_ROOT/classified.json"

for name in unique-out unique-in outside-out outside-in ambiguous-out ambiguous-in-a ambiguous-in-b beyond-out beyond-in classified; do
  printf '%s=' "$name"
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/$name.json"
done | tee "$E2E_ROOT/fixture-ids.txt"
UNIQUE_OUT_ID=$(sed -n 's/^unique-out=//p' "$E2E_ROOT/fixture-ids.txt")
UNIQUE_IN_ID=$(sed -n 's/^unique-in=//p' "$E2E_ROOT/fixture-ids.txt")

curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/classification-rules" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"matchType\":\"exact\",\"condition\":\"classified\",\"categoryId\":\"$CATEGORY_ID\"}" \
   >"$E2E_ROOT/classification-rule.json"
```

### Completion-gate fixture matrix

Create the following additional rows before submitting any matching command. The
selected matching range later is `[2026-06-01T00:00:00-04:00,
2026-06-03T00:00:00-04:00)`. Each amount is unique to its named case except
where the case intentionally creates ambiguity.

```bash
# Same-account rows must never pair.
create_fixture "$TENANT_ID" "$ACCOUNT_A" -1100 'same-account-out' '2026-06-02T08:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/same-account-out.json"
create_fixture "$TENANT_ID" "$ACCOUNT_A" 1100 'same-account-in' '2026-06-02T09:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/same-account-in.json"

# The only opposite for this tenant-A row lives in a different tenant.
OTHER_TENANT_ID=$(curl -sS -X POST http://127.0.0.1:4501/api/v1/finance/tenants \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"name\":\"transfer-matching-other-$RUN_ID\",\"displayCurrency\":\"USD\",\"seedDefaults\":false}" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
OTHER_ACCOUNT_ID=$(curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$OTHER_TENANT_ID/accounts" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"name":"Other tenant checking","currency":"USD","kind":"manual"}' |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
create_fixture "$TENANT_ID" "$ACCOUNT_A" -1200 'cross-tenant-out' '2026-06-02T10:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/cross-tenant-out.json"
create_fixture "$OTHER_TENANT_ID" "$OTHER_ACCOUNT_ID" 1200 'cross-tenant-in' '2026-06-02T11:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/cross-tenant-in.json"

# Every one of these candidates is ineligible, so its otherwise-valid partner
# must also remain unpaired.
create_fixture "$TENANT_ID" "$ACCOUNT_A" -1300 'pending-out' '2026-06-02T08:00:00-04:00' USD pending regular '' '[]' >"$E2E_ROOT/pending-out.json"
create_fixture "$TENANT_ID" "$ACCOUNT_B" 1300 'pending-in' '2026-06-02T09:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/pending-in.json"
create_fixture "$TENANT_ID" "$ACCOUNT_A" -1400 'refund-out' '2026-06-02T08:00:00-04:00' USD booked refund '' '[]' >"$E2E_ROOT/refund-out.json"
create_fixture "$TENANT_ID" "$ACCOUNT_B" 1400 'refund-in' '2026-06-02T09:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/refund-in.json"

# The public create contract intentionally rejects amountMinor zero. Prove that
# contract first; it must not be relaxed to support this verification fixture.
ZERO_PUBLIC_BODY=$(python3 - "$ACCOUNT_A" <<'PY'
import json, sys
print(json.dumps({
    "accountId": sys.argv[1], "source": "manual", "status": "booked",
    "kind": "regular", "amountMinor": 0, "currency": "USD",
    "description": "zero-public-rejection",
    "effectiveAt": "2026-06-02T08:00:00-04:00", "tagIds": [],
}))
PY
)
printf '%s\n' "$ZERO_PUBLIC_BODY" >"$E2E_ROOT/zero-public-request.json"
ZERO_PUBLIC_STATUS=$(curl -sS -o "$E2E_ROOT/zero-public-response.json" -w '%{http_code}' \
  -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "$ZERO_PUBLIC_BODY")
test "$ZERO_PUBLIC_STATUS" = 400

# Only the two zero-amount eligibility rows bypass public creation. This uses the
# documented local PostgreSQL runtime role (not owner or migrator), tenant and
# account IDs created above, and one tenant-scoped INSERT ... SELECT per row. It
# is valid only after this guide's isolated/reseeded `make postgres-bootstrap`.
# All matching and verification below still use product HTTP/worker surfaces.
create_zero_fixture() {
  local account_id="$1" description="$2" effective_at="$3" transaction_id row_id
  transaction_id=$(python3 -c 'import uuid; print(uuid.uuid4())')
  row_id=$(printf '%s\n' "INSERT INTO finance_transactions (id, tenant_id, account_id, source, status, kind, amount_minor, currency, description, effective_at, transfer_matching_excluded, created_at, updated_at)
          SELECT :'transaction_id', a.tenant_id, a.id, 'manual', 'booked', 'regular', 0, 'USD', :'description', (:'effective_at')::timestamptz, false, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
          FROM finance_accounts AS a
          WHERE a.id = :'account_id' AND a.tenant_id = :'tenant_id'
          RETURNING id;" | docker compose exec -T -e PGPASSWORD=sumweave_runtime postgres \
    psql -X -v ON_ERROR_STOP=1 -qAt -p 55432 -U sumweave_runtime -d sumweave_local \
      -v transaction_id="$transaction_id" -v tenant_id="$TENANT_ID" \
      -v account_id="$account_id" -v description="$description" -v effective_at="$effective_at")
  test "$row_id" = "$transaction_id"
  printf '{"id":"%s"}\n' "$transaction_id" >"$E2E_ROOT/$description.json"
}
create_zero_fixture "$ACCOUNT_A" 'zero-a' '2026-06-02T08:00:00-04:00'
create_zero_fixture "$ACCOUNT_B" 'zero-b' '2026-06-02T09:00:00-04:00'

# Link one pair manually. Link then unlink a second pair, which is the supported
# way to create the hidden automatic-matching exclusion without exposing it.
create_fixture "$TENANT_ID" "$ACCOUNT_A" -1500 'already-linked-out' '2026-06-02T08:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/already-linked-out.json"
create_fixture "$TENANT_ID" "$ACCOUNT_B" 1500 'already-linked-in' '2026-06-02T09:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/already-linked-in.json"
create_fixture "$TENANT_ID" "$ACCOUNT_A" -1600 'excluded-out' '2026-06-02T08:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/excluded-out.json"
create_fixture "$TENANT_ID" "$ACCOUNT_B" 1600 'excluded-in' '2026-06-02T09:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/excluded-in.json"
ALREADY_LINKED_OUT_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/already-linked-out.json")
ALREADY_LINKED_IN_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/already-linked-in.json")
EXCLUDED_OUT_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/excluded-out.json")
EXCLUDED_IN_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/excluded-in.json")
create_manual_link "$ALREADY_LINKED_OUT_ID" "$ALREADY_LINKED_IN_ID" >"$E2E_ROOT/already-linked.json"
create_manual_link "$EXCLUDED_OUT_ID" "$EXCLUDED_IN_ID" >"$E2E_ROOT/excluded-link.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions/$ALREADY_LINKED_OUT_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/before-already-linked-out.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions/$ALREADY_LINKED_IN_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/before-already-linked-in.json"
curl -sS -X DELETE "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions/transfer-links" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"firstTransactionId\":\"$EXCLUDED_OUT_ID\",\"secondTransactionId\":\"$EXCLUDED_IN_ID\"}" >"$E2E_ROOT/excluded-unlink.json"

# Exact integer amounts and currency must agree; either sign may be the starting leg.
create_fixture "$TENANT_ID" "$ACCOUNT_A" -1700 'unequal-out' '2026-06-02T08:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/unequal-out.json"
create_fixture "$TENANT_ID" "$ACCOUNT_B" 1699 'unequal-in' '2026-06-02T09:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/unequal-in.json"
create_fixture "$TENANT_ID" "$ACCOUNT_A" -1800 'cross-currency-out' '2026-06-02T08:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/cross-currency-out.json"
create_fixture "$TENANT_ID" "$ACCOUNT_EUR" 1800 'cross-currency-in' '2026-06-02T09:00:00-04:00' EUR booked regular '' '[]' >"$E2E_ROOT/cross-currency-in.json"
create_fixture "$TENANT_ID" "$ACCOUNT_A" 1900 'positive-start-in' '2026-06-02T12:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/positive-start-in.json"
create_fixture "$TENANT_ID" "$ACCOUNT_B" -1900 'positive-start-outside' '2026-06-03T12:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/positive-start-outside.json"

# These two legs are loaded as outer evidence but neither is in the original range.
create_fixture "$TENANT_ID" "$ACCOUNT_A" -2000 'two-outside-out' '2026-05-30T12:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/two-outside-out.json"
create_fixture "$TENANT_ID" "$ACCOUNT_B" 2000 'two-outside-in' '2026-05-31T12:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/two-outside-in.json"

# One-to-many and many-to-one must remain ambiguous, not choose a nearest leg.
create_fixture "$TENANT_ID" "$ACCOUNT_A" -2100 'many-to-one-out-a' '2026-06-02T08:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/many-to-one-out-a.json"
create_fixture "$TENANT_ID" "$ACCOUNT_C" -2100 'many-to-one-out-b' '2026-06-02T09:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/many-to-one-out-b.json"
create_fixture "$TENANT_ID" "$ACCOUNT_B" 2100 'many-to-one-in' '2026-06-02T10:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/many-to-one-in.json"

# A at hour 0 is in-range, B is exactly hour 72, and C is hour 144. C is
# outer ambiguity evidence for B, so neither A/B nor B/C may be created.
create_fixture "$TENANT_ID" "$ACCOUNT_A" -2200 'hour-0-out' '2026-06-01T00:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/hour-0-out.json"
create_fixture "$TENANT_ID" "$ACCOUNT_B" 2200 'hour-72-in' '2026-06-04T00:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/hour-72-in.json"
create_fixture "$TENANT_ID" "$ACCOUNT_C" -2200 'hour-144-out' '2026-06-07T00:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/hour-144-out.json"

# One pair is classified before matching; another is classified only after matching.
create_fixture "$TENANT_ID" "$ACCOUNT_A" -2300 'classify-before' '2026-06-02T14:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/classify-before-out.json"
create_fixture "$TENANT_ID" "$ACCOUNT_B" 2300 'classify-before' '2026-06-02T15:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/classify-before-in.json"
create_fixture "$TENANT_ID" "$ACCOUNT_A" -2400 'classify-after' '2026-06-02T16:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/classify-after-out.json"
create_fixture "$TENANT_ID" "$ACCOUNT_B" 2400 'classify-after' '2026-06-02T17:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/classify-after-in.json"

# More than one list page cannot alter matching: these unilateral rows create
# pagination pressure without introducing opposite-value candidates.
for n in $(seq 1 101); do
  create_fixture "$TENANT_ID" "$ACCOUNT_C" "$((30000 + n))" "page-filler-$n" '2026-06-02T18:00:00-04:00' USD booked regular '' '[]' >/dev/null
done
create_fixture "$TENANT_ID" "$ACCOUNT_A" -2500 'page-filter-out' '2026-06-02T18:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/page-filter-out.json"
create_fixture "$TENANT_ID" "$ACCOUNT_B" 2500 'page-filter-in' '2026-06-02T19:00:00-04:00' USD booked regular '' '[]' >"$E2E_ROOT/page-filter-in.json"
```

Before matching, classify the `classify-before` pair through the supported
explicit classification route. This is deliberately a separate bounded delivery
before the matching submission.

```bash
curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/classification-rules" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"matchType\":\"exact\",\"condition\":\"classify-before\",\"categoryId\":\"$CATEGORY_ID\"}" >"$E2E_ROOT/classify-before-rule.json"
curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions/classify" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"rangeStart":"2026-06-02T14:00:00-04:00","rangeEndExclusive":"2026-06-02T16:00:00-04:00"}' >"$E2E_ROOT/classify-before-trigger.json"
CLASSIFY_BEFORE_JOB_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["jobId"])' <"$E2E_ROOT/classify-before-trigger.json")
CLASSIFY_BEFORE_STATUS=$(curl -sS -o "$E2E_ROOT/classify-before-before-delivery.json" -w '%{http_code}' \
  "http://127.0.0.1:4501/api/v1/jobs/$CLASSIFY_BEFORE_JOB_ID" -H "Authorization: Bearer $ACCESS_TOKEN")
test "$CLASSIFY_BEFORE_STATUS" = 404
go run ./cmd/sumweave jobs worker --once --env local >"$E2E_ROOT/worker-classify-before.log" 2>&1
curl -sS "http://127.0.0.1:4501/api/v1/jobs/$CLASSIFY_BEFORE_JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/classify-before-job.json"
test "$(python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])' <"$E2E_ROOT/classify-before-job.json")" = succeeded
```

Before matching, capture the ledger and a fixed custom June reporting range. The
RFC 3339 bounds cover every fixture timestamp through the June 7 outer row; do
not use `current_month` or date-only values because the guide can run in any
calendar month.

```bash
JUNE_DASHBOARD_QUERY='startDate=2026-06-01T00%3A00%3A00-04%3A00&endDate=2026-06-08T00%3A00%3A00-04%3A00'
printf '%s\n' "$JUNE_DASHBOARD_QUERY" >"$E2E_ROOT/june-dashboard-query.txt"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions?limit=100" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/ledger-before.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/dashboard?$JUNE_DASHBOARD_QUERY" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/dashboard-before.json"
```

Verify in the saved bodies that the categorized/tagged fixture has those IDs,
the account balances include the native values, and the candidates are still
regular. Do not infer a match from a transaction-list page: the matcher is
tenant-wide and the list endpoint is paged.

Also submit one valid empty DST-crossing range, such as local `2026-03-07` to
`2026-03-08`, and retain its request body. It must carry
`2026-03-07T00:00:00-05:00` and `2026-03-09T00:00:00-04:00`; it has no fixture
rows and therefore proves only boundary serialization. Keep reversed or invalid
date requests out of the API run: the browser/unit checks reject them before
publication.

## 3. Explicit range, API-only `404`, and bounded delivery

Capture a deliberately filtered and offset ledger page before matching. It does
not drive the match request; it proves later that an account/page filter cannot
turn an ambiguous or off-page candidate into a unique one. Then submit the
narrow original range. It includes the in-range starting legs but not the
outside partners, and is half-open even though the UI displays inclusive dates.

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions?accountId=$ACCOUNT_A&status=booked&source=manual&limit=1&offset=100" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/filtered-offset-page-before.json"
MATCH_REQUEST='{"rangeStart":"2026-06-01T00:00:00-04:00","rangeEndExclusive":"2026-06-03T00:00:00-04:00"}'
printf '%s\n' "$MATCH_REQUEST" >"$E2E_ROOT/explicit-request.json"
curl -sS -X POST \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions/match-transfers" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "$MATCH_REQUEST" \
  >"$E2E_ROOT/explicit-trigger.json"
MATCH_JOB_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["jobId"])' <"$E2E_ROOT/explicit-trigger.json")
MATCH_STATUS=$(curl -sS -o "$E2E_ROOT/explicit-before-delivery.json" -w '%{http_code}' \
  "http://127.0.0.1:4501/api/v1/jobs/$MATCH_JOB_ID" -H "Authorization: Bearer $ACCESS_TOKEN")
test "$MATCH_STATUS" = 404

go run ./cmd/sumweave jobs worker --once --env local >"$E2E_ROOT/worker-explicit.log" 2>&1
curl -sS "http://127.0.0.1:4501/api/v1/jobs/$MATCH_JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/explicit-job.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions?limit=100" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/ledger-after-explicit.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/dashboard?$JUNE_DASHBOARD_QUERY" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/dashboard-after-explicit.json"

fetch_fixture() {
  local name="$1" id
  id=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/$name.json")
  curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions/$id" \
    -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/after-$name.json"
}
for name in unique-out unique-in outside-out outside-in positive-start-in positive-start-outside \
  classify-before-out classify-before-in classify-after-out classify-after-in page-filter-out page-filter-in \
  already-linked-out already-linked-in excluded-out excluded-in same-account-out same-account-in \
  pending-out pending-in refund-out refund-in zero-a zero-b unequal-out unequal-in \
  cross-currency-out cross-currency-in two-outside-out two-outside-in beyond-out beyond-in ambiguous-out ambiguous-in-a ambiguous-in-b \
  many-to-one-out-a many-to-one-out-b many-to-one-in hour-0-out hour-72-in hour-144-out; do
  fetch_fixture "$name"
done
CROSS_TENANT_IN_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/cross-tenant-in.json")
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions/$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$E2E_ROOT/cross-tenant-out.json")" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/after-cross-tenant-out.json"
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/$OTHER_TENANT_ID/transactions/$CROSS_TENANT_IN_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/after-cross-tenant-in.json"

assert_transfer_pair() {
  python3 - "$E2E_ROOT/after-$1.json" "$E2E_ROOT/after-$2.json" <<'PY'
import json, sys
a, b = (json.load(open(path)) for path in sys.argv[1:])
assert a["kind"] == b["kind"] == "transfer"
assert a["transferGroupId"] and a["transferGroupId"] == b["transferGroupId"]
assert a["transferMatchedAt"] and b["transferMatchedAt"]
PY
}
assert_unpaired() {
  python3 - "$1" "$2" "$3" <<'PY'
import json, sys
path, kind, status = sys.argv[1:]
row = json.load(open(path))
assert row["kind"] == kind and row["status"] == status
assert row.get("transferGroupId") is None and row.get("transferMatchedAt") is None
PY
}
assert_category() {
  python3 - "$1" "$2" <<'PY'
import json, sys
assert json.load(open(sys.argv[1]))["categoryId"] == sys.argv[2]
PY
}
```

Assert all of the following from the saved bodies:

- `explicit-job.json` has `id == MATCH_JOB_ID`, job type
  `finance.transfer-matching`, and terminal `succeeded` or a documented
  sanitized `failed` result. A passing fixture run expects `succeeded`.
- `explicit-request.json` has only the two range fields: no account, status,
  source, query, offset, page, or list filter is sent to matching.
- Run `assert_transfer_pair unique-out unique-in`,
  `assert_transfer_pair outside-out outside-in`,
  `assert_transfer_pair positive-start-in positive-start-outside`, and
  `assert_transfer_pair page-filter-out page-filter-in`. These cover exact 72
  elapsed hours, a partner outside the original range, a positive in-range
  starting leg, and a pair created despite the filtered/offset list page.
- Run `assert_transfer_pair classify-before-out classify-before-in` and
  `assert_category "$E2E_ROOT/after-classify-before-out.json" "$CATEGORY_ID"`
  (and the equivalent `classify-before-in` assertion). Classification before
  matching therefore does not block the pair or erase its category.
- Run `assert_transfer_pair already-linked-out already-linked-in`; its existing
  group/timestamp remains a valid manual link. Run
  `assert_unpaired "$E2E_ROOT/after-excluded-out.json" regular booked` and the
  equivalent `excluded-in` assertion: the supported link/unlink exclusion keeps
  those otherwise compatible rows unpaired without exposing an exclusion field.
- Run `assert_unpaired` for `same-account-out`/`same-account-in`,
  `cross-tenant-out` and `cross-tenant-in`, `pending-out`/`pending-in`,
  `refund-out`/`refund-in`, `zero-a`/`zero-b`, `unequal-out`/`unequal-in`, and
  `cross-currency-out`/`cross-currency-in`, using their original kinds/statuses
  (`refund-out` is `refund/booked`; `pending-out` is `regular/pending`; all
  other listed rows are `regular/booked`). This proves same-account and
  cross-tenant isolation plus pending, refund, zero, unequal-amount, and
  cross-currency rejection.
- Run `assert_unpaired` for every `ambiguous-*`, every `many-to-one-*`, and
  `hour-0-out`, `hour-72-in`, and `hour-144-out` as `regular/booked`. The latter
  is the exact hour-0/hour-72/hour-144 outer-ambiguity boundary: the hour-144
  row prevents a false hour-0/hour-72 pair. The ambiguous rows remain unpaired
  even though the filtered list did not include all candidates.
- Run `assert_unpaired` for `two-outside-out` and `two-outside-in` as
  `regular/booked`: both legs are outer loaded evidence, not starting rows, so
  the original range is not widened. Run the same assertion for `beyond-out`
  and `beyond-in`, which are just beyond 72 elapsed hours.
- categorized/tagged `unique-out` keeps its category and tag IDs; all fixtures'
  account IDs, signed amounts, currencies, effective timestamps, descriptions,
  and account balance effects are unchanged.
- compare the dashboard snapshots: matched internal transfers remain reflected
  in account balances but do not remain in settled income/expense totals.

Run this complete assertion batch; it is independent of the ordering or contents
of `filtered-offset-page-before.json` and uses only individual fixture reads.

```bash
assert_transfer_pair unique-out unique-in
assert_transfer_pair outside-out outside-in
assert_transfer_pair positive-start-in positive-start-outside
assert_transfer_pair page-filter-out page-filter-in
assert_transfer_pair classify-before-out classify-before-in
assert_transfer_pair already-linked-out already-linked-in
assert_category "$E2E_ROOT/after-classify-before-out.json" "$CATEGORY_ID"
assert_category "$E2E_ROOT/after-classify-before-in.json" "$CATEGORY_ID"
python3 - "$E2E_ROOT/before-already-linked-out.json" "$E2E_ROOT/after-already-linked-out.json" \
  "$E2E_ROOT/before-already-linked-in.json" "$E2E_ROOT/after-already-linked-in.json" \
  "$E2E_ROOT/after-unique-out.json" "$CATEGORY_ID" "$TAG_ID" <<'PY'
import json, sys
before_out, after_out, before_in, after_in, unique_out = (json.load(open(path)) for path in sys.argv[1:6])
for before, after in ((before_out, after_out), (before_in, after_in)):
    assert before["transferGroupId"] == after["transferGroupId"]
    assert before["transferMatchedAt"] == after["transferMatchedAt"]
assert unique_out["categoryId"] == sys.argv[6]
assert sys.argv[7] in unique_out["tagIds"]
PY
for name in same-account-out same-account-in cross-tenant-out cross-tenant-in \
  pending-out pending-in refund-out refund-in zero-a zero-b unequal-out unequal-in \
  cross-currency-out cross-currency-in excluded-out excluded-in two-outside-out two-outside-in \
  beyond-out beyond-in ambiguous-out ambiguous-in-a ambiguous-in-b many-to-one-out-a \
  many-to-one-out-b many-to-one-in hour-0-out hour-72-in hour-144-out; do
  case "$name" in
    pending-out) assert_unpaired "$E2E_ROOT/after-$name.json" regular pending ;;
    refund-out) assert_unpaired "$E2E_ROOT/after-$name.json" refund booked ;;
    cross-tenant-in) assert_unpaired "$E2E_ROOT/after-$name.json" regular booked ;;
    *) assert_unpaired "$E2E_ROOT/after-$name.json" regular booked ;;
  esac
done
python3 - "$E2E_ROOT/dashboard-before.json" "$E2E_ROOT/dashboard-after-explicit.json" <<'PY'
import json, sys
before, after = (json.load(open(path)) for path in sys.argv[1:])
assert before["period"]["startDate"] == after["period"]["startDate"]
assert before["period"]["endDate"] == after["period"]["endDate"]
assert before["accountBalances"] == after["accountBalances"]
assert after["settled"]["incomeMinor"] < before["settled"]["incomeMinor"]
assert after["settled"]["expenseMinor"] < before["settled"]["expenseMinor"]
PY
```

The pre-delivery `404` is valid only for `MATCH_JOB_ID` from this initiating
request. A copied, arbitrary job URL must retain normal not-found behavior.

## 4. Classification after matching, rerun, and manual correction

Now classify the already matched `classify-after` pair. Classification must skip
transfer kinds, so the rule must not add a category or remove the transfer link.

```bash
curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/classification-rules" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"matchType\":\"exact\",\"condition\":\"classify-after\",\"categoryId\":\"$CATEGORY_ID\"}" >"$E2E_ROOT/classify-after-rule.json"
curl -sS -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions/classify" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data '{"rangeStart":"2026-06-02T16:00:00-04:00","rangeEndExclusive":"2026-06-02T18:00:00-04:00"}' >"$E2E_ROOT/classify-after-trigger.json"
CLASSIFY_AFTER_JOB_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["jobId"])' <"$E2E_ROOT/classify-after-trigger.json")
go run ./cmd/sumweave jobs worker --once --env local >"$E2E_ROOT/worker-classify-after.log" 2>&1
curl -sS "http://127.0.0.1:4501/api/v1/jobs/$CLASSIFY_AFTER_JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" >"$E2E_ROOT/classify-after-job.json"
test "$(python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])' <"$E2E_ROOT/classify-after-job.json")" = succeeded
fetch_fixture classify-after-out
fetch_fixture classify-after-in
assert_transfer_pair classify-after-out classify-after-in
python3 - "$E2E_ROOT/after-classify-after-out.json" "$E2E_ROOT/after-classify-after-in.json" <<'PY'
import json, sys
assert all(json.load(open(path)).get("categoryId") is None for path in sys.argv[1:])
PY
```

This complements the pre-match classification fixture: a category assigned
before matching survives, while classification after matching preserves the
transfer and does not classify it.

Submit the same explicit range again while API-only, assert its new job ID is
different, run the bounded worker, and save both job/ledger responses. The
second pass must leave already-paired records unchanged.

Unlink the accepted unique pair through the supported manual-correction API:

```bash
curl -sS -X DELETE \
  "http://127.0.0.1:4501/api/v1/finance/tenants/$TENANT_ID/transactions/transfer-links" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  --data "{\"firstTransactionId\":\"$UNIQUE_OUT_ID\",\"secondTransactionId\":\"$UNIQUE_IN_ID\"}" \
  >"$E2E_ROOT/unlink.json"
```

After a further explicit rerun, both unlinked legs must remain regular and
unpaired. Then manually link the same excluded legs with the supported
`POST .../transactions/transfer-links` endpoint and verify the pair is allowed
again. The API intentionally has no exclusion indicator; the later browser
check must remain equally silent.

## 5. Real automatic committed-window coverage

Use [synthetic-provider-flow-e2e.md](./synthetic-provider-flow-e2e.md) only to
create a local synthetic connection and publish one real committed window over
the prepared matching date range. Keep the classification rule created above and
the `classified` fixture unpaired.

With API-only still running, publish the synthetic sync, record its initial
bank-job `404`, then run `jobs worker --once`. Save the bank job, ledger, and
dashboard bodies. The bounded worker drains observed commands first and then
both ordinary enrichment routers; no order between classification and matching
is implied.

Verify:

- the synthetic bank job succeeds and creates the committed window;
- matching processes the tenant-wide manual fixture when its starting leg is in
  that window, with no separate `finance.transfer-matching` observed job;
- classification assigns the `classified` fixture independently;
- the category already on a matched fixture remains untouched regardless of
  classification timing; and
- replaying the same supported sync and bounded drain leaves completed pairs
  unchanged.

The public API does not expose a way to redeliver one existing appdispatch
message or inject a later pair-write failure. Record those assertions from the
focused automated coverage rather than modifying database state by hand:

```bash
(cd "$REPO_ROOT/finance" && go test . -run 'TestTransferMatchingService/(retains_an_earlier_PostgreSQL_pair_when_a_later_pair_save_fails|preserves_classification,_tags,_balances,_reporting,_and_fresh_pending_eligibility)') \
  | tee "$E2E_ROOT/partial-failure-and-independence-test.log"
(cd "$REPO_ROOT/apps/sumweave" && go test ./internal/financeapp -run 'TestFinance(JobRegistrationAdapters|RegistrationPostgres)') \
  | tee "$E2E_ROOT/redelivery-and-window-delivery-test.log"
```

These records establish the supported failure/retry behavior: an earlier
committed pair survives a later failure, a fresh rerun reloads remaining
eligibility, and duplicate delivery is safe. Do not claim a manually injected
failure or redelivery when no supported API/harness performed one.

The synthetic connector also has no supported business-failure response. The
Finance-app test record is the supported evidence that a later sync-window
failure cannot erase an earlier committed event or ledger writes; do not mutate
dispatch tables manually to manufacture that condition.

## 6. Headed browser ledger smoke

After the API checks, restore the documented PM2 API/worker/UI setup. Run this
headed check at `1280x900` and `390x844`; store screenshots and any Playwright
trace under `$E2E_ROOT/browser/`.

1. Sign in, choose the fixture tenant, and open `#/finance/transactions`.
2. Confirm **Match transfers** is Bootstrap-first, shows today and the prior 29
   local calendar dates, and exposes both inclusive dates before submission.
3. Enter a wider historical range containing the fixture. Confirm the exact
   tenant-wide scope copy and that account, status, source, sort, and page
   filters do not change the matching range submission.
4. With the worker stopped, submit and confirm only this initiating job can show
   the waiting-for-worker state; retain **Open finance job**.
5. Start a bounded worker, observe queued/running and terminal feedback, then
   confirm the ledger refreshes after either terminal state. Success must say
   `Transfer matching completed.` Failure must say `Transfer matching failed.
   Some pairs may already have been matched. Running it again preserves existing
   pairs.`
6. Open a matched record, perform the existing unlink/manual-link correction,
   and confirm no automatic-exclusion warning or indicator appears.
7. At both viewport sizes, confirm date inputs, action, job link, filters, and
   ledger rows remain visible, usable, and free of overlap, clipping, or
   horizontal overflow. Also complete the relevant paths in
   [finance-ui-shell-smoke-e2e.md](./finance-ui-shell-smoke-e2e.md).

## 7. Cleanup and report

```bash
cleanup_api_only 0
trap - EXIT
```

Report the run directory, tenant/job IDs, selected ranges, terminal jobs,
fixture assertions, viewport screenshots, and any failed API/worker/browser
step. Keep evidence in `tmp/`; do not commit it.
