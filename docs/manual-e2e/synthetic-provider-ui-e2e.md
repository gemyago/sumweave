# Synthetic Provider UI Manual E2E

Follow preparation steps in [README.md](./README.md) first.

This guide covers the browser flow only: sign in, create or reuse an active finance tenant, start synthetic setup from Finance connections, save pending configuration, reload it, finish the link, and confirm the new synthetic connection card appears.

## 1. Sign in

1. Open `http://127.0.0.1:5173/#/login`.
2. Sign in with the first local user from repo-root `.local-users` unless you intentionally prepared another user.
3. Confirm the app redirects to a protected route instead of staying on `#/login`.

Expected:

- the login succeeds
- you land inside the authenticated UI shell

## 2. Prepare an active finance tenant

If you already have a clean tenant for this run, reuse it. Otherwise create one:

1. Open `#/finance/tenants`.
2. In **Create tenant**, enter a unique name such as `synthetic-ui-e2e-<yyyymmdd-hhmmss>`.
3. Set **Display currency** to `USD`.
4. Click **Create tenant**.
5. In **Selected tenant**, choose the tenant you want to use for the run.

Expected:

- the tenant appears in the selected-tenant picker
- the Finance subnav shows the chosen tenant name

## 3. Start synthetic setup from Finance connections

1. Open `#/finance/connections`.
2. Confirm the correct tenant is selected in the **Tenant** picker.
3. In **Configure synthetic provider**, click **Start synthetic setup**.

Expected:

- the browser stays in the app and navigates to `#/finance/connections/synthetic?state=...`
- the **Synthetic setup** page shows a non-empty pending setup state
- the setup route keeps the same selected tenant

## 4. Save duplicate configured accounts

1. On the synthetic setup page, keep the first configured row and set:
   - **Account name 1** = `Synthetic Checking`
   - **Account currency 1** = `USD`
2. Click **Add account**.
3. Set the second row to the same values:
   - **Account name 2** = `Synthetic Checking`
   - **Account currency 2** = `USD`
4. Click **Save configuration**.

Expected:

- the page shows `Configuration saved.`
- the page changes from `Save at least one configured account before finishing.` to `Pending setup can be finished.`
- both duplicate rows remain visible after save

## 5. Reload the pending setup

1. Click **Reload pending setup**.
2. Confirm both configured rows are still present.

Expected:

- the page stays on the same `state`
- both duplicate rows survive reload as separate rows
- the route does not drop back to Finance connections

## 6. Finish the link

1. Click **Finish link**.

Expected:

- the browser returns to `#/finance/connections`
- the selected tenant stays the same
- a new **Synthetic** connection card appears
- the new card shows `synthetic · active`
- the secondary line includes `Provider ref: <state>`

## 7. Optional follow-up checks

If you want to continue beyond the browser setup smoke test:

1. Click **Sync now** on the new Synthetic card.
2. Use [synthetic-provider-flow-e2e.md](./synthetic-provider-flow-e2e.md) to verify linked accounts and provider transactions over the API.

## 8. If anything is wrong, report it

Capture:

- the selected tenant name
- the synthetic setup `state` from the URL
- screenshots or snapshots of save/reload/finish failures
- any inline alert text shown on the setup or connections page
