Durable visual acceptance for all tasks: keep finance shell chrome calm, make the first viewport read as compact header plus period context, primary balance story, compact income/expense/pending summaries, then a primary visual summary before recent activity or attention states, and keep any tenant switcher compact and utility-like rather than page-dominant.

## 1. Tenant And Shell Simplification

- [x] 1.1 Update finance shell tenant tests first to prove a single joined tenant is auto-selected without rendering a visible tenant selector on tenant-scoped finance routes, including direct `#/finance`, `#/finance/accounts`, and `#/finance/transactions` entry, then implement the hidden single-tenant selector behavior while keeping workspace chrome calm and secondary to dashboard content.
- [x] 1.2 Update multi-tenant finance shell tests first to prove exactly one compact shell-owned workspace switcher appears, dashboard content does not render duplicate tenant picker or tenant workspace blocks, and selecting another tenant reloads tenant-scoped finance data without route loss; keep the switcher compact and utility-like rather than a large page-level control.
- [x] 1.3 Add failing cross-route tenant-context tests first for both auto-selected single-tenant entry and previously selected multi-tenant entry, covering direct deep-link access and shared active-tenant reuse across `#/finance`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/connections`, `#/finance/imports`, and `#/finance/jobs/:jobId`, then implement any missing shell/state wiring so the active tenant carries across those routes until the operator changes it.

## 2. Dashboard Information Architecture

- [x] 2.1 Add failing dashboard tests first for the simplified first viewport: compact header, period context, primary balance summary, income, expense, pending delta, and no large tenant/reporting control panel; the tested first viewport must make the money story obvious before secondary controls.
- [x] 2.2 Replace the current dashboard hero, controls panel, KPI strip, and review-queue-first layout with the simplified dashboard hierarchy using existing dashboard, transaction, account, category, and connection data sources; the new layout must preserve a clear card rhythm, visual-first chart placement, and two-column desktop composition without inventing unsupported widgets.
- [x] 2.3 Add failing tests first for capped account, category, and recent-transaction dashboard sections, then cap repeated dashboard rows and link full detail to Accounts or Transactions routes so those sections stay scannable instead of expanding into long repeated detail lists.

## 3. Attention And Diagnostics

- [x] 3.1 Add failing tests first for compact needs-attention rendering of pending transactions, missing FX, failed sync, or failed import signals, then demote operational diagnostics from primary dashboard cards into the attention strip so they stay secondary to balance, spending, cash-flow, and recent-activity panels.
- [x] 3.2 Ensure admin-specific FX diagnostics links are not primary dashboard actions, while still keeping reachable admin diagnostics from existing admin routes and avoiding promotion of operator/admin diagnostics into the main visual hierarchy.

## 4. Responsive And Visual Follow-Through

- [x] 4.1 Add or update responsive tests where practical, then adjust mobile finance shell/dashboard layout so navigation and tenant chrome do not push the dashboard summary below the first viewport while preserving the same balance-first priority order when the desktop rail collapses.
- [x] 4.2 Update `apps/signal-ui/ui-wireframe.md` and finance manual smoke guidance to match the simplified dashboard and conditional tenant switcher behavior, and capture the durable visual acceptance criteria in written form rather than by pointing to a temporary repo asset.
- [x] 4.3 Run the finance UI smoke guide, visual assessment, and required repository lint/test completion protocol after implementation; compare the result against the written acceptance criteria in this change for clarity, card rhythm, first-viewport usefulness, cross-route tenant consistency, and tenant-control restraint, then fix any findings and rerun until clean.
