# Synthetic Provider UI Manual E2E

Follow preparation steps in [README.md](./README.md) first.

This guide covers the browser flow only: sign in, create or reuse an active finance tenant, start synthetic setup from Finance connections, save pending configuration, reload it, finish the link, and confirm the new synthetic connection card appears. It also prepares the explicit classification panel for the API-only deterministic delivery checks.

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

## 7. Prepare explicit classification feedback

1. Open `#/finance/rules` and confirm the **Run classification** panel appears before the rule editor.
2. Confirm the default visible date range is today and the preceding 29 calendar dates.
3. Choose a short date range that includes a known uncategorized fixture transaction and select **Run classification**.
4. Confirm the panel identifies the returned Finance job. In the normal PM2 setup, the worker can materialize it before the first UI poll, so do not expect an initial `404`.
5. To deterministically observe the initiating-request `404`, use the API-only worker stop/start window in [synthetic-provider-flow-e2e.md](./synthetic-provider-flow-e2e.md#explicit-classification-and-category-preservation-checks). Its isolated setup stops PM2 backend processes, starts only the API, asserts the `404` after submission, then runs the bounded worker.
6. Start or wait for the local worker, then confirm the panel shows terminal success or an actionable failure message. On success, open `#/finance/transactions` and confirm the matching ledger row shows its category without a manual reload.

Expected:

- date inputs use local calendar dates and include the selected final date
- the job link is limited to the classification request just submitted
- only the initiating request treats a not-yet-observed `404` as pending
- failed jobs give a retry-oriented message; partial failures identify them as partial
- successful classification refreshes the ledger data for the selected tenant

Use the deterministic fixture and exact request assertions in
[synthetic-provider-flow-e2e.md](./synthetic-provider-flow-e2e.md) for a full
automatic-classification, explicit-classification, and category-preservation
run. Do not run that destructive API-only guide while another developer needs
the local database.

## 8. Optional follow-up checks

If you want to continue beyond the browser setup smoke test:

1. Click **Sync now** on the new Synthetic card.
2. Open **Synced accounts** on that card and confirm only account name, currency, last successful sync, and in-app account links are shown; close/reopen to confirm it uses the per-card cache until a new sync invalidates it.
3. Use [synthetic-provider-flow-e2e.md](./synthetic-provider-flow-e2e.md) to verify linked accounts and provider transactions over the API.

## 9. If anything is wrong, report it

Capture:

- the selected tenant name
- the synthetic setup `state` from the URL
- screenshots or snapshots of save/reload/finish failures
- any inline alert text shown on the setup or connections page
- the classification start/end dates, job link, visible job state, and ledger category
