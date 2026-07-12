# Report Finance Transaction Through UI Manual E2E

Follow the preparation steps in [README.md](./README.md) first. This guide uses the browser UI; use the first local user from repo-root `.local-users`.

Use a unique run tag such as `YYYYMMDD-HHMMSS` in the tenant, account, and transaction descriptions. This keeps the created records easy to identify without deleting existing local data.

## 1. Sign in

1. Open `http://127.0.0.1:5173/#/login` in the browser.
2. Sign in with the first `username:password` entry from repo-root `.local-users`.
3. Confirm the app opens `#/finance` and no login request fails.

Expected:

- sign-in succeeds
- the authenticated Finance shell is visible
- the browser console has no errors related to authentication or Finance route loading

## 2. Create and select a fresh tenant

1. Open `#/finance/tenants` from the Finance navigation.
2. In **Create tenant**, enter `transaction-ui-e2e-<run-tag>` for **Name**.
3. Choose `USD` for **Display currency**.
4. Select **Create tenant**.
5. Choose the new run-tagged tenant from **Selected tenant**.
6. Confirm the new tenant is selected in the Finance shell.

Expected:

- the create-tenant request succeeds
- the selected tenant shows the unique run-tagged name
- tenant-scoped routes keep this tenant selected

## 3. Create a manual account

1. Open `#/finance/accounts`.
2. In **Create account**, enter `Transaction UI account <run-tag>` for **Name**.
3. Enter `USD` for **Currency**.
4. Choose `manual` for **Kind**.
5. Select **Create account**.
6. Confirm the account appears in **Account list**.

Expected:

- the create-account request succeeds
- the new account is visible under the selected tenant
- its displayed currency is `USD`
- it is not labeled **Hidden**

## 4. Report a transaction

1. Open `#/finance/transactions`.
2. Select **Create transaction**.
3. Confirm the browser opens `#/finance/transactions/new` and shows **Record transaction**.
4. Fill the editor with:
   - **Account**: `Transaction UI account <run-tag>`
   - **Category**: `No category`
   - **Kind**: `expense`
   - **Status**: `booked`
   - **Source**: `manual`
   - **Currency**: `USD`
   - **Amount minor**: `-1234`
   - **Transfer group**: empty
   - **Description**: `Transaction UI e2e <run-tag>`
   - **Effective at**: a valid local date and time
5. Select **Save transaction** once.

Expected:

- one `POST /api/v1/finance/tenants/<tenant-id>/transactions` request is sent
- the request returns `200`
- the page shows **Transaction recorded.**
- the editor remains usable and shows the persisted transaction values

## 5. Verify the ledger row

1. Select **Back to transactions**.
2. Confirm the browser opens `#/finance/transactions`.
3. If needed, select the run-tagged account in **Account** and keep **Status** set to `Any status`.
4. Find the row whose description is `Transaction UI e2e <run-tag>`.
5. Confirm the row shows the run-tagged account, `-12.34 USD`, `booked`, and `manual`.
6. Open the row's **Edit** action and confirm the dedicated edit route loads the same description, amount, and effective date.

Expected:

- the created transaction appears exactly once
- the amount, account, status, source, and description match the submitted form
- the transaction detail request succeeds and the edit route preserves the selected tenant

## 6. Verify account and dashboard effects

1. Open `#/finance/accounts` and find `Transaction UI account <run-tag>`.
2. Confirm its booked balance is `-12.34 USD` and its pending balance is `0.00 USD`.
3. Open `#/finance` and keep the dashboard period set to the current month.
4. Confirm the settled summary shows:
   - **Expense**: `12.34 USD`
   - **Settled net**: `-12.34 USD`
   - **Booked transactions**: `1`
5. Confirm the run-tagged account shows a booked balance of `-12.34 USD` and a pending balance of `0.00 USD` on the dashboard.
6. In **Recent activity**, find exactly one entry whose description is `Transaction UI e2e <run-tag>`.
7. Confirm that entry is a booked expense for `-12.34 USD` and shows the same effective local date and time submitted in the editor.

Expected:

- the account list API returns `bookedBalanceMinor=-1234` and `pendingBalanceMinor=0` for the run-tagged account
- the current-month dashboard API returns settled `expenseMinor=1234`, `netMinor=-1234`, and `transactionCount=1`
- the current-month dashboard API reports the run-tagged account's booked balance as `-1234` and pending balance as `0`
- pending dashboard totals remain zero with `transactionCount=0`
- the visible account balances and settled dashboard totals match the API values
- recent activity contains the created transaction exactly once with the exact description, status, kind, amount, and effective time

## 7. If anything is wrong, report it

Stop at the first product failure and capture:

- signed-in username and selected tenant/account names
- exact route and browser viewport
- the form values used
- visible alert, stale state, or navigation behavior
- failed or unexpected network request, status, request payload, and response body
- browser console errors or warnings
- whether the transaction appears in the ledger after reloading `#/finance/transactions`
- the run-tagged account's displayed and API booked/pending balances
- the current-month dashboard's displayed settled expense, settled net, and booked transaction count
- the dashboard API period, settled and pending totals, and run-tagged account balance payload
- the displayed recent-activity entry and its corresponding API record, including description, status, kind, amount, and effective time
