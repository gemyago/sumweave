## Context

The current `/finance` screen has real sandbox data, but the hierarchy is inverted:

- global finance chrome and route chrome take too much space
- tenant selection is visible in multiple places
- reporting controls dominate the first viewport
- review queue, FX diagnostics, and sync language appear at the same level as money summaries
- chart cards include explanatory copy that makes the dashboard read like implementation documentation

The visual target is not a literal style copy. The durable product pattern is:

- calm finance navigation
- short dashboard header
- one dominant balance or money story
- compact income and expense summaries
- visual cash flow and spending/category panels
- recent transactions below the primary summary
- secondary operational status only when attention is required

## Goals / Non-Goals

**Goals:**

- Make the first viewport answer "how am I doing financially?"
- Use existing finance sandbox data and current finance API clients.
- Remove duplicate tenant controls from dashboard content.
- Hide tenant switching for single-tenant users.
- Keep multi-tenant switching available but compact and shell-owned.
- Demote operational diagnostics from primary dashboard cards.
- Preserve the current Signal UI design tokens unless the implementation explicitly updates reusable UI guidance.
- Update `apps/signal-ui/ui-wireframe.md` alongside implementation.

**Non-Goals:**

- No new backend dashboard API.
- No new persisted user preference or tenant model.
- No fake notification, avatar, advice, goals, settings, or budgeting features from the reference image.
- No design-system rewrite away from the terminal-native foundation in this change.
- No changes to non-finance routes.

## Decisions

1. The dashboard starts with product finance summaries, not workspace explanations.

   - Replace the large hero copy with a compact dashboard title, active period, and primary actions.
   - Copy should use product language such as "Track spending, cash flow, and accounts" rather than implementation language such as "tenant-aware finance routes".
   - The first viewport should contain balance, income, expense, and at least one primary visual summary whenever data is available.

2. Tenant selection is shell-owned and conditional.

   - If the user has exactly one joined tenant, the shell should auto-select it and not show a tenant selector on normal tenant-scoped finance routes.
   - If the user has multiple tenants, the shell should show one compact workspace switcher.
   - After the active tenant is resolved, the shell should reuse that workspace across `#/finance`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/connections`, `#/finance/imports`, and `#/finance/jobs/:jobId`, including direct entry by deep link.
   - Dashboard panels must not repeat selected-tenant labels or tenant picker blocks.
   - `/finance/tenants` remains the route for tenant creation, invites, members, and explicit tenant administration.

3. Reporting controls become compact dashboard controls.

   - Current, previous, and next period actions should be available near the dashboard header or relevant cards.
   - Custom date range should move behind a compact filter/details affordance instead of occupying a large full-width panel.
   - The visible period summary remains clear and synchronized with the dashboard response.

4. Operational diagnostics become attention states.

   - Missing FX, pending transactions, failed sync, and failed import information should be shown as a compact "Needs attention" strip or secondary panel.
   - Admin-specific deep links, such as FX diagnostics, should not be a primary dashboard action.
   - If there are no attention items, the dashboard should avoid rendering a large empty diagnostics card.

5. Dashboard sections are capped and route-oriented.

   - Spending/category, account, and transaction sections should show only the most useful visible rows.
   - Full lists stay on dedicated Accounts and Transactions routes.
   - The dashboard should not render both a chart and a long repeated exact-value list for the same concept in the primary area.

6. Responsive behavior should show dashboard value before chrome.

    - Desktop may keep the finance rail.
    - Mobile should avoid pushing the dashboard below navigation, tenant, and explanatory chrome.
    - Mobile finance navigation should collapse into compact tabs or a route menu so the dashboard summary appears early.

7. Visual acceptance should stay durable and written.

   - Implementation and follow-up docs should validate against written criteria, not a temporary asset path.
   - The first viewport should read in this order whenever data is available: compact header and period context, primary balance story, compact income/expense/pending summaries, one primary visual summary, then recent activity and attention states.
   - Finance shell chrome should stay visually secondary to the dashboard content.
   - Tenant switching, when present, should stay compact and utility-like rather than becoming a large page-level control.

## Proposed Dashboard Shape

Top area:

- Dashboard title
- active period selector
- primary action to add or open transactions
- optional compact workspace switcher only for multi-tenant users

Primary grid:

- balance or net-worth style summary from account balances
- income summary
- expense summary
- pending delta

Analytics:

- cash-flow visual from settled and pending income/expense data
- spending by category visual from category breakdown data
- account snapshot capped to top accounts by magnitude

Activity:

- recent transactions capped to a small list
- needs-attention strip for pending, missing FX, failed sync, or import state

## Risks / Trade-offs

- Hiding the tenant selector for single-tenant users may make tenant concepts less discoverable.
  - Mitigation: keep tenant management discoverable through `/finance/tenants` and show a workspace switcher only when switching is useful.
- Existing tests may expect route-level tenant fields.
  - Mitigation: update tests to assert shell-owned tenant behavior instead of duplicated page chrome.
- The current dashboard API may not expose a perfect "balance" aggregate.
  - Mitigation: derive a balance summary from existing account balance data in the UI until a later backend change is justified.
- A terminal-native visual system can look sparse if only colors change.
  - Mitigation: improve hierarchy through layout, spacing, type scale, icon affordances, and reduced copy before changing the broader design system.

## Migration Plan

1. Update finance dashboard tests to describe the simplified first viewport and shell-owned tenant behavior.
2. Refactor finance shell tenant presentation for single-tenant and multi-tenant modes, including shared active-tenant reuse across tenant-scoped routes and direct finance deep links.
3. Replace the dashboard controls panel with compact period controls.
4. Replace dashboard cards with balance, cash-flow, spending, account snapshot, recent transactions, and needs-attention sections.
5. Update responsive shell/dashboard behavior so mobile reaches the dashboard summary quickly.
6. Update `apps/signal-ui/ui-wireframe.md` and the finance manual smoke guide.
7. Run the UI smoke loop, visual assessment, and repo completion checks required for UI code changes.

Because the project is early alpha, the existing dashboard can be removed instead of preserved behind compatibility flags.

## Open Questions

- Should "balance" mean booked account balance only, or booked plus pending as a separate displayed delta? The proposed first implementation should show booked balance as primary and pending as a separate delta unless implementation data reveals a clearer existing convention.
