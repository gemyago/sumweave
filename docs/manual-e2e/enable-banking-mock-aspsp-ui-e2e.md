# Enable Banking Mock ASPSP UI Manual E2E

Follow preparation steps in [README.md](./README.md) first.

This guide covers the browser flow for PKO linking through Enable Banking sandbox when the configured sandbox ASPSP is `Mock ASPSP`. It also covers how to prepare mock accounts and transactions before authorizing access.

## 1. Open the browser

Use the default browser mode unless one of these is true:

- the user explicitly asked for headed mode
- the agent hit a problem that needs human intervention and the visible browser helps the user fix it

Default command:

```bash
npx playwright-cli open "http://127.0.0.1:5173/#/login"
```

If headed mode is justified, use:

```bash
npx playwright-cli open --headed "http://127.0.0.1:5173/#/login"
```

Expected:

- the app opens on `#/login`
- if headed mode was used, the browser window is visible

## 2. Sign in to Signal Foundry

1. Sign in with the first local user from repo-root `.local-users` unless you intentionally prepared another user.
2. Open `http://127.0.0.1:5173/#/finance/connections`.
3. Select or create the tenant you want to use for the run.

Expected:

- the Finance connections page shows the selected tenant
- **Connect PKO with bank login** is enabled

## 3. Start PKO linking

1. On Finance connections, click **Connect PKO with bank login**.
2. On the Enable Banking consent page, click **Continue authentication**.
3. If Enable Banking asks for email sign-in, use the primary AgentMail inbox from repo-root `.agentmail`.
4. Read the Enable Banking email with AgentMail and open the one-time sign-in link in the same browser session.

Expected:

- Enable Banking signs in as the primary AgentMail email
- the browser reaches the Mock ASPSP authorization route

## 4. Prepare mock ASPSP accounts

If the authorization page reports `No Account`, or if the run requires adding new accounts, click **Create Account** or open `https://enablebanking.com/cp/mock-aspsp` in the same signed-in browser session to manage accounts.

To add an account:

1. In **Add a new account**, set **Name** to a readable test name such as `PKO E2E Sample 1`.
2. Keep the generated JSON template unless the test needs specific account fields.
3. Click **Add**.
4. Repeat until the account list has the required number of accounts.

Expected:

- the **Accounts** table lists each mock account
- each row has **Name**, **Cash Account Type**, **Currency**, and **Balances & Transactions**

## 5. Add mock ASPSP transactions

Mock transactions are managed per mock account.

1. On `https://enablebanking.com/cp/mock-aspsp`, click **Balances & Transactions** for the account you want to seed.
2. In **Balances**, keep or edit the generated balance JSON template, then click the **Add** button in the Balances section.
3. In **Transactions**, prefer the generated transaction JSON template for most runs, then click the **Add** button in the Transactions section.
4. Edit transaction JSON only when the test needs specific `transactionAmount`, `creditDebitIndicator`, `status`, `bookingDate`, `valueDate`, or `remittanceInformation` values.
5. Repeat for each account that should produce synced transaction data.
6. Return to `https://enablebanking.com/cp/mock-aspsp` when done.

Example booked debit transaction JSON:

```json
{
  "entryReference": "sf-e2e-tx-001",
  "transactionAmount": {
    "currency": "EUR",
    "amount": "12.34"
  },
  "creditor": {
    "name": "Signal Foundry Merchant 1"
  },
  "creditDebitIndicator": "DBIT",
  "status": "BOOK",
  "bookingDate": "2026-07-07",
  "valueDate": "2026-07-07",
  "remittanceInformation": ["Signal Foundry test transaction 1"]
}
```

Expected:

- each seeded account keeps its configured balances or transactions
- the authorization screen still lists the seeded accounts

## 6. Authorize all required accounts

Return to the active authorization URL. If you navigated away, use the URL from the current run, for example:

```text
https://enablebanking.com/cp/mock-aspsp/auth?redirect_url=...&valid_until=...&state=...
```

1. Confirm the authorization page lists all accounts required for the run.
2. Check every account that should be linked.
3. Click **Authorize**.

Expected:

- Enable Banking redirects through `tilisy-sandbox.enablebanking.com`
- the browser returns to `http://127.0.0.1:5173/#/finance/connections`
- a `Mock ASPSP` connection appears with provider `pko` and state `active`

## 7. Trigger sync

1. On Finance connections, click **Sync now** on the new `Mock ASPSP` card.
2. Click **Open latest finance job**.
3. Confirm the job status.

Expected:

- the job type is `finance.bank_connection_sync`
- the job reaches `succeeded`

## 8. Optional app checks

After a successful sync, inspect the tenant data in the app:

1. Open `#/finance/accounts` to check linked accounts.
2. Open `#/finance/transactions` to check synced transactions.
3. If expected provider data is missing despite a succeeded sync job, capture the connection provider reference, job id, selected tenant, and visible account or transaction counts.

## 9. If anything is wrong, report it

Capture:

- selected Signal Foundry tenant name
- Enable Banking signed-in email
- Mock ASPSP account names selected during authorization
- returned connection provider reference
- sync job id and status
- any inline alert text shown by Signal Foundry or Enable Banking
