# Synthetic Provider Flow Manual E2E

Follow preparation steps in [README.md](./README.md) and get the API token before starting.

This guide is mostly API-only. The only non-API step is the synthetic link itself because the synthetic provider is still core-only and does not yet have a public HTTP linking route.

## 1. Create a fresh finance tenant

Use a unique tenant name on each run.

```bash
RUN_TAG=$(date +%Y%m%d-%H%M%S) && TENANT_NAME="synthetic-provider-e2e-${RUN_TAG}" && CREATE_STATUS=$(curl -sS -o /tmp/synthetic-provider-tenant-create.json -w "%{http_code}" -X POST "http://127.0.0.1:4501/api/v1/finance/tenants" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data "{\"name\":\"${TENANT_NAME}\",\"displayCurrency\":\"USD\"}") && test "$CREATE_STATUS" = "200" && TENANT_ID=$(python3 -c 'import json; print(json.load(open("/tmp/synthetic-provider-tenant-create.json"))["id"])') && printf 'tenantId=%s\n' "$TENANT_ID"
```

Expected:

- `200`
- response includes a new tenant `id`

## 2. Resolve the authenticated user id

The synthetic link helper needs the real backend user id, not only the username.

```bash
ME_STATUS=$(curl -sS -o /tmp/synthetic-provider-auth-me.json -w "%{http_code}" "http://127.0.0.1:4501/api/v1/auth/me" -H "Authorization: Bearer ${ACCESS_TOKEN}") && test "$ME_STATUS" = "200" && USER_ID=$(python3 -c 'import json; print(json.load(open("/tmp/synthetic-provider-auth-me.json"))["id"])') && printf 'userId=%s\n' "$USER_ID"
```

Expected:

- `200`
- response includes `id` and `username`

## 3. Link one synthetic connection

The command below writes a temporary Go test file under `finance/`, runs it once, prints the created connection id, and removes the file again.

```bash
ACCOUNT_NAME="Synthetic Checking ${RUN_TAG}" && cat > finance/manual_synthetic_provider_link_tmp_test.go <<'EOF'
package finance

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	internalsynthetic "github.com/gemyago/signal-foundry/finance/internal/synthetic"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/stretchr/testify/require"
)

func TestManualSyntheticProviderLink(t *testing.T) {
	repoRoot := os.Getenv("SIGNAL_FOUNDRY_REPO_ROOT")
	userID := os.Getenv("SYNTHETIC_E2E_USER_ID")
	tenantID := os.Getenv("SYNTHETIC_E2E_TENANT_ID")
	accountName := os.Getenv("SYNTHETIC_E2E_ACCOUNT_NAME")
	require.NotEmpty(t, repoRoot)
	require.NotEmpty(t, userID)
	require.NotEmpty(t, tenantID)
	require.NotEmpty(t, accountName)

	database, err := persistence.OpenDatabase(filepath.Join(repoRoot, "apps/signal-foundry/data/data-layer.db"))
	require.NoError(t, err)
	store := persistence.NewStore(database)

	jwtKey, err := os.ReadFile(filepath.Join(repoRoot, "apps/signal-foundry/data/auth/jwt-signing-key"))
	require.NoError(t, err)
	key := sha256.Sum256(jwtKey)
	cipher, err := credentials.NewAESGCMCipher(key[:], "signal-foundry-finance")
	require.NoError(t, err)

	syntheticStateStore := persistence.NewSyntheticProviderStateStoreFromStore(store)
	connector := internalsynthetic.NewConnector(syntheticStateStore)
	provider, ok := newConnectorBankSyncProvider(connector)
	require.True(t, ok)

	service := NewService(store, WithConnectionSecretCipher(cipher), WithBankProviders(provider))
	syncStore, err := service.bankSyncStore()
	require.NoError(t, err)

	linker := internalsynthetic.NewLinker(internalsynthetic.LinkerDeps{
		RequireTenantMember: service.requireTenantMember,
		SaveConnectionSecret: func(ctx context.Context, providerName, reference, secret string) (string, error) {
			return service.encryptAndSaveConnectionSecret(ctx, providerName, reference, secret)
		},
		DeleteConnectionSecret: store.DeleteConnectionSecret,
		SaveBankConnection:     syncStore.SaveBankConnection,
		DeleteBankConnectionOwnedMetadata: func(ctx context.Context, connection domain.BankConnection) error {
			return service.deleteBankConnectionOwnedMetadata(ctx, connection)
		},
		SaveSyntheticProviderState: syntheticStateStore.SaveSyntheticProviderState,
		Now:                        service.now,
		NewID:                      service.newID,
	})

	connection, err := linker.LinkConfiguredBankConnection(t.Context(), internalsynthetic.LinkConfiguredBankConnectionParams{
		ActorUserID: userID,
		TenantID:    tenantID,
		Provider:    "synthetic",
		Accounts: []internalsynthetic.ConfiguredAccount{{
			Name:     accountName,
			Currency: "USD",
		}},
	})
	require.NoError(t, err)
	t.Logf("connection_id=%s", connection.ID)
	t.Logf("account_name=%s", accountName)
}
EOF
SIGNAL_FOUNDRY_REPO_ROOT="$PWD" SYNTHETIC_E2E_USER_ID="$USER_ID" SYNTHETIC_E2E_TENANT_ID="$TENANT_ID" SYNTHETIC_E2E_ACCOUNT_NAME="$ACCOUNT_NAME" go test ./finance -run TestManualSyntheticProviderLink -count=1 -v | tee /tmp/synthetic-provider-link.out && CONNECTION_ID=$(python3 -c 'import pathlib,re; text=pathlib.Path("/tmp/synthetic-provider-link.out").read_text(); print(re.search(r"connection_id=(\S+)", text).group(1))') && rm -f finance/manual_synthetic_provider_link_tmp_test.go && printf 'connectionId=%s\n' "$CONNECTION_ID"
```

Expected:

- `go test` passes
- output contains `connection_id=<uuid>`

## 4. Verify the linked synthetic connection exists before sync

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections" -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Expected:

- `200`
- one item matches `${CONNECTION_ID}`
- that item has `provider="synthetic"`, `displayName="Synthetic"`, and `state="active"`

## 5. Trigger one manual sync

Use a fixed window so repeated checks are easy to compare.

```bash
SYNC_STATUS=$(curl -sS -o /tmp/synthetic-provider-sync-trigger.json -w "%{http_code}" -X POST "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections/${CONNECTION_ID}/sync" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json" --data '{"reason":"manual","windowStart":"2026-06-01T00:00:00Z","windowEnd":"2026-06-04T00:00:00Z"}') && test "$SYNC_STATUS" = "200" && python3 -c 'import json; data=json.load(open("/tmp/synthetic-provider-sync-trigger.json")); assert data["jobId"]; assert data["jobType"]=="finance.bank_connection_sync"; print("jobId=" + data["jobId"])'
```

Expected:

- `200`
- response includes `jobId`
- response `jobType` is `finance.bank_connection_sync`

## 6. Wait for sync completion

The sync runs asynchronously. Poll the connection until `lastSuccessfulSyncAt` is no longer the zero timestamp.

```bash
for attempt in 1 2 3 4 5 6 7 8 9 10; do curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/connections" -H "Authorization: Bearer ${ACCESS_TOKEN}" > /tmp/synthetic-provider-connections-after-sync.json && CONNECTION_ID="$CONNECTION_ID" python3 - <<'PY'
import json, os, sys
connection_id = os.environ["CONNECTION_ID"]
items = json.load(open('/tmp/synthetic-provider-connections-after-sync.json'))['items']
item = next((x for x in items if x['id'] == connection_id), None)
assert item is not None, 'connection missing during poll'
if item.get('lastSuccessfulSyncAt') and item['lastSuccessfulSyncAt'] != '0001-01-01T00:00:00Z':
    print(item['lastSuccessfulSyncAt'])
    sys.exit(0)
sys.exit(1)
PY
if [ $? -eq 0 ]; then break; fi
sleep 2
done
```

Expected:

- the poll exits successfully within a few attempts
- the connection now shows non-zero `lastSyncStartedAt` and `lastSuccessfulSyncAt`

## 7. Verify the linked account and provider transactions

List accounts:

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/accounts" -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

List provider transactions:

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/transactions?source=provider" -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Optional one-shot assertion:

```bash
curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/accounts" -H "Authorization: Bearer ${ACCESS_TOKEN}" > /tmp/synthetic-provider-accounts.json && curl -sS "http://127.0.0.1:4501/api/v1/finance/tenants/${TENANT_ID}/transactions?source=provider" -H "Authorization: Bearer ${ACCESS_TOKEN}" > /tmp/synthetic-provider-transactions.json && ACCOUNT_NAME="$ACCOUNT_NAME" python3 - <<'PY'
import json, os
account_name = os.environ['ACCOUNT_NAME']
accounts = json.load(open('/tmp/synthetic-provider-accounts.json'))['items']
transactions = json.load(open('/tmp/synthetic-provider-transactions.json'))['items']
assert len(accounts) == 1, accounts
account = accounts[0]
assert account['name'] == account_name, account
assert account['provider'] == 'synthetic', account
assert account['providerAccountId'], account
assert len(transactions) > 0, transactions
assert all(item['source'] == 'provider' for item in transactions), transactions
assert all(item['accountId'] == account['id'] for item in transactions), transactions
assert all(item.get('providerOriginal') for item in transactions), transactions
print(f"accountId={account['id']}")
print(f"bookedBalanceMinor={account['bookedBalanceMinor']}")
print(f"providerTransactions={len(transactions)}")
PY
```

Expected:

- account list returns one linked account for this flow
- linked account has `provider="synthetic"` and non-empty `providerAccountId`
- provider transaction list is non-empty
- provider transactions all point to the linked account and include `providerOriginal`

## 8. If anything is wrong, report it

Capture:

- tenant id and connection id
- the link helper output
- sync trigger response
- connection list after sync
- account list response
- transaction list response
