# Finance transaction CSV import E2E

## Purpose

Verify the fixed transaction CSV preview, confirmation, durable progress, and results.

## Setup

1. Follow the setup in [README.md](./README.md).
2. Sign in with the first `.local-users` entry.
3. Create or select an isolated Finance tenant.
4. Open `#/finance/imports`.

## Preview

1. Confirm the page lists these seven required headers:
   `Date,Account,Category,Tags,Expense amount,Income amount,Currency`, plus optional
   `Description`.
2. Download or copy the sample, then preview this CSV:

```csv
Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description
29.05.26,CSV checking,Groceries,"home, food","8 300,00",,PLN,"Monthly groceries"
30.05.26,CSV checking,Salary,"work, income",,"12 500,50",PLN,"May salary"
```

3. Verify the page explains the 250,000-data-row (header excluded), 64 MiB CSV limit; strict `dd.MM.yy` dates, `00`–`99` mapping to 2000–2099, USD/EUR/PLN/UAH support, quoted tags, and localized quoted amounts. Verify the seven required headers and optional Description may be in any order, blank/missing descriptions become `n/a`, and unsupported extra columns are ignored.
4. Verify the preview shows `Transactions to import: 2` along with account, category, and tag creation summaries. It must not show import-type or mapping controls. Verify the Step 2 **Accounts to include** checkbox group lists both detected textual accounts, source-row counts, and has every account checked initially.
5. Click **Preview transactions** and immediately verify its pending label is visible, the button is disabled against a duplicate request, and the active Step 2 workspace directly below Step 1 is brought into view/focus before the result appears.
6. Add rows with an invalid date, both amount columns populated, and an unsupported currency. Preview again and verify each rejected row includes its row number, source field when available, and reason.
7. Uncheck one account and verify `Updating preview…` appears without moving focus, then the replacement preview excludes that account's rows, diagnostics, duplicate checks, and account/category/tag creation summaries. Re-check it and verify the source-row count remains stable. Uncheck every account and verify the replacement preview reports `Transactions to import: 0` and disables **Confirm valid rows**; it must not silently restore all accounts.
8. Add an invalid row under one detected account and verify its diagnostic appears while that account is selected but disappears when that account is unchecked. Blank/unassignable account rows may remain diagnostics.
9. If a duplicate row is reported, verify it also shows row/field/reason, the preview count excludes rejected/duplicate rows, and the page says they are excluded from confirmation. Preview only invalid/duplicate rows and verify `Transactions to import: 0`.

## Confirm and audit

1. Confirm valid rows. In browser network tools, verify the initial preview sends only `fileName` and `csv`, replacement previews send the same source plus `selectedAccountNames`, and confirm has no request body. Verify confirmation imports only checked-account rows and does not create accounts, categories, or tags for unchecked accounts.
2. Verify the audit initially reports running/confirmed progress when the job has not completed, then refreshes automatically or with **Refresh audit**.
3. Verify the audit reaches `completed` or shows a recoverable `failed` state, lists final rejected rows and row outcomes, and links to `#/finance/jobs/:jobId`.
4. Open the job detail link and verify its status agrees with the import audit.
5. At a completed audit, verify **Confirm valid rows** is no longer available. Reopen the same audit from **Recent imports**, including after visiting the job detail and returning to Imports; verify it remains scoped to the selected tenant and retains its job/outcomes.
6. When **Open audit** is clicked, verify only the selected history item becomes visibly loading/disabled, then the active workspace above Recent imports is scrolled/focused and shows the selected audit. Leave a running audit open long enough to confirm background refresh does not steal focus.
7. Open Finance accounts, transactions, and categories/tags. Verify imported transactions use the expected PLN values, account currency, categories, and readable tags.

## Narrow-screen check

At a 390×844 viewport, verify file selection, sample actions, textarea, account checkboxes, preview diagnostics, audit outcomes, and job link remain readable without horizontal page overflow.
