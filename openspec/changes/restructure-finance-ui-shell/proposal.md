## Why

- Finance routes currently feel like extensions of the generic operator shell: a shared horizontal nav, an in-page finance subnav card, and repeated tenant pickers on each page.
- The supplied reference points to a finance-first information architecture with a persistent rail, compact utility chrome, a dashboard hierarchy, and a transactions workspace that feels purpose-built for day-to-day finance operations.
- We need a pragmatic planning slice that adopts that structure for supported finance routes without rewriting the whole design system, inventing dead routes, or expanding backend scope just to mimic the screenshot.

## What Changes

- Plan a dedicated finance shell for `#/finance*` with a persistent left rail on desktop, compact top utility row, and shared shell-level tenant context.
- Recompose `#/finance` into a finance dashboard hierarchy that follows the reference layout priorities while preserving the repo's terminal-native design tokens and honest data states.
- Recompose `#/finance/transactions` into a table-first browse workspace with search/date/filter controls, summary chips, row selection, and a responsive contextual inspector while keeping dedicated create/edit routes.
- Keep unsupported reference items such as `Rules`, `Settings`, fake notification/help affordances, or other dead links out of scope until backed by real product workflows.
- Add planning/spec artifacts, update the future UI wireframe task scope, and add a manual finance-shell smoke runbook plus implementation iteration expectations.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-operator-ui`: adopt a finance-first shell, dashboard hierarchy, and transactions browse workspace for the existing finance routes.

## Impact

- Affected implementation areas: `apps/signal-ui`, `apps/signal-ui/ui-wireframe.md`, finance UI tests, `docs/manual-e2e/`, and the `finance-operator-ui` OpenSpec artifacts.
- Planned implementation should derive dashboard and transactions behavior from existing finance endpoints first; this change does not propose new Go or API scope.
- Finance routes will see meaningful layout churn, but non-finance routes remain out of scope.
