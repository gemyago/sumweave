## Context

- `apps/signal-ui` already imports Bootstrap globally and contains a Bootstrap pilot for login and the Finance dashboard.
- Canonical Finance routes still use the older route set and product rules that describe Bootstrap as temporary or isolated.
- The requested rollout changes the decision boundary: Finance should be Bootstrap by default, while the rest of Signal UI remains on the existing stack for now.
- Finance app scope for this change means tenant-facing `#/finance*` routes. Finance-related admin diagnostics under `#/admin/*` remain part of the broader Admin surface unless a later request moves them into Finance.
- Active OpenSpec overlap is resolved in favor of this change: `adopt-finance-bootstrap-default` supersedes `restructure-finance-ui-shell` for Finance/login styling direction, while allowing behavior-only reuse from its shared shell and tenant-context work.

## Goals / Non-Goals

**Goals:**

- Make canonical `#/login` the Bootstrap login route.
- Send successful login and root/default authenticated routing to `#/finance` unless a protected destination was remembered.
- Make canonical `#/finance*` routes render in a Bootstrap-first Finance shell with Bootstrap-first page composition.
- Cover dashboard, tenants, accounts, account detail, transactions, transaction create/edit, categories, connections, synthetic setup, imports, and finance job detail.
- Reuse existing Finance API clients, auth/session state, tenant workspace behavior, formatting helpers, and current data contracts.
- Reuse only behavior-level lessons from `restructure-finance-ui-shell`, such as route coverage, route-preserving tenant selection, and supported navigation mapping, not its custom styling direction.
- Replace module docs/rules that describe Bootstrap as temporary with rules that define Bootstrap as the Finance default.

**Non-Goals:**

- No Go/backend/OpenAPI work unless implementation discovers a true route-contract mismatch.
- No Bootstrap conversion for Chat, Data, generic Jobs, Providers, Strategies, Evaluations, Admin, or other non-finance surfaces.
- No Svelte Bootstrap wrapper, second utility CSS framework, or route-local bespoke CSS as the primary Finance styling model.
- No compatibility preservation for the prior parallel pilot route names; the project is early alpha.
- No new Finance workflows, fake navigation, unsupported notification/help affordances, or backend-only dashboard widgets.

## Decisions

1. Bootstrap becomes canonical only at the Finance and login boundaries.

   - Canonical Finance routes should use Bootstrap classes and native Svelte/HTML as their primary visual API.
   - Non-finance pages keep the current `DESIGN.md`/design-system behavior so this rollout does not become a full app rewrite.
   - Alternative considered: convert the whole SPA to Bootstrap now. Rejected because the user explicitly asked to leave the rest of the system on the old stack for now.

2. Promote behavior into canonical routes instead of keeping parallel product names.

   - `#/login` and `#/finance*` become the accepted product surface.
   - Existing pilot components can be renamed, moved, or folded into canonical components, but user-facing route/product names should not carry pilot terminology.
   - Legacy `#/v2/login` and `#/v2/finance` routes should not remain registered as compatibility-only product surfaces, and tests/docs/rules should not preserve them without explicit user approval.
   - Alternative considered: redirect canonical routes to the pilot routes. Rejected because the request is for Bootstrap as the default with no pilot naming.

3. Use one Bootstrap Finance shell for all tenant-facing Finance routes.

   - The shell should own Finance navigation, active tenant context, sign out, theme control, responsive containment, and page content slots.
   - Every supported `#/finance*` route should render inside the same shell, including detail/editor/job/synthetic routes.
   - Implementation may reuse behavior-only shell state helpers from the current Finance shell or from `restructure-finance-ui-shell` work.
   - The prior custom/terminal-native shell styling from `restructure-finance-ui-shell` is superseded and should be replaced with Bootstrap-first composition for canonical Finance routes.

4. Supersede old custom-shell styling while preserving useful behavior lessons.

   - `restructure-finance-ui-shell` remains useful as implementation reference for shared Finance route wiring, route-preserving tenant resolution, compact tenant-control behavior, supported Finance destination mapping, and avoiding unsupported placeholder links.
   - It does not define the visual acceptance target for this change. Canonical `#/login` and tenant-facing `#/finance*` surfaces should not preserve the older custom-shell or terminal-native styling direction as their primary styling model.
   - No separate manager/user blocker is required before implementation; implementation should follow this change as the current product direction.

5. Port surfaces by route groups rather than by visual widget type.

   - Login and routing/default destination should land first because they determine entry behavior.
   - Shell and canonical dashboard should land before the remaining Finance pages.
   - Transactions/editor should be treated as a focused slice because it has browse, detail handoff, and mutation-state complexity.

6. Keep API and data behavior stable.

   - Dashboard and page content should continue deriving from existing finance dashboard, account, transaction, category, connection, import, tenant, and job helpers.
   - If a Bootstrap layout wants a widget with no existing source, render an honest reduced/empty state or omit it rather than adding backend scope.

7. Final-review responsive correction should document the shell that exists unless a later product request asks for a new mobile menu interaction.

   - The implemented Bootstrap Finance shell uses a desktop left rail and a stacked narrow layout where the Finance nav becomes a full-width Bootstrap aside above the route content.
   - The correction round should update `ui-wireframe.md` and the manual smoke guide to describe that behavior instead of claiming an explicit narrow-screen menu toggle.
   - Adding a new responsive toggle is out of scope for this correction round unless the current stacked shell fails the responsive smoke acceptance.

## Risks / Trade-offs

- Superseding `restructure-finance-ui-shell` can leave already-changed Finance shell code in a custom-styled intermediate state → treat that code as behavior reference only and replace Finance/login visuals with Bootstrap-first composition during the ordered implementation slices.
- Bootstrap global CSS can bleed into old-stack routes → scope custom bridge styles carefully and test representative non-finance routes for obvious regressions.
- A route-by-route Finance rewrite can regress tenant deep links → require tests for tenant selection and route preservation on list/detail/editor/job/synthetic routes.
- Reusing pilot components may leak old product naming into code/tests/docs → require renames or canonical wrappers during implementation.
- Login default changes operator muscle memory from Data to Finance → keep remembered protected destinations honored so explicit deep links still land where requested.

## Migration Plan

1. Treat `adopt-finance-bootstrap-default` as superseding `restructure-finance-ui-shell` for Finance/login styling, with behavior-only reuse limited to shell route coverage, tenant resolution, and supported navigation lessons.
2. Update canonical login/default routing tests and route constants so fallback authenticated navigation lands on `#/finance`.
3. Promote the Bootstrap login surface into `#/login` and remove pilot-specific login route/product naming from the planned product surface.
4. Replace canonical Finance shell wiring with a Bootstrap Finance shell for every supported `#/finance*` route.
5. Promote/adapt the Bootstrap dashboard into canonical `#/finance`.
6. Convert remaining Finance route groups to Bootstrap-first markup and states while preserving current API behavior.
7. Remove any remaining legacy `#/v2/login` or `#/v2/finance` route registration, protected-destination handling, tests, rules, and docs that preserve compatibility-only behavior.
8. Update docs, AGENTS rules, wireframe, and manual smoke guides to describe Bootstrap Finance as canonical, describe the actual stacked narrow Finance shell behavior, and keep the rest of the app as unchanged.

Rollback is not a planning requirement because this repository is early alpha and the request explicitly promotes the new direction in place.

## Open Questions

- No blockers. Planning assumes Finance app means tenant-facing `#/finance*` routes, while `#/admin/*` remains old-stack Admin for this change. The active overlap with `restructure-finance-ui-shell` is resolved by this change superseding the older custom-shell styling direction and limiting reuse to behavior-only shell/tenant lessons.
