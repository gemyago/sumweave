## Context

The existing Signal UI design system is custom, terminal-native, and classed through local `ds-*` primitives plus route-specific CSS. That direction has produced too many small visual decisions for agents to maintain reliably. The login page and finance dashboard are the right pilot surfaces because they are visible, bounded, and representative of the two main UI shapes: auth form and dense finance summary.

Migrating those surfaces in place would force the Bootstrap experiment to share the old route shell, visual assumptions, and tests from day one. A parallel `#/v2/*` route space gives the pilot a clean boundary: agents can treat V2 pages as Bootstrap-only while the existing routes remain available for comparison and rollback.

There is an active `restructure-finance-ui-shell` change that preserves the current terminal-native custom design system. This change intentionally avoids competing with that implementation in place. It can still reuse the useful information-architecture ideas from that change, but Bootstrap becomes the styling source of truth for the parallel V2 pilot routes.

## Goals / Non-Goals

**Goals:**

- Make Bootstrap 5 CSS the default styling layer for the pilot.
- Add parallel `#/v2/login` and `#/v2/finance` pilot routes.
- Use native Svelte/HTML markup with Bootstrap classes directly.
- Avoid page-local bespoke CSS in V2 login and V2 finance dashboard components.
- Keep custom CSS limited, named, and documented.
- Update agent rules so future UI edits inside the pilot do not drift back into bespoke CSS.
- Preserve existing auth and finance dashboard data behavior.
- Keep canonical `#/login` and `#/finance` stable until a later promotion decision.
- Verify visually on desktop and mobile before marking implementation tasks complete.

**Non-Goals:**

- No backend, OpenAPI, auth contract, or finance API changes.
- No whole-app Bootstrap migration in the first implementation chunk.
- No redirecting canonical routes to V2 in this change.
- No Bootstrap JavaScript component dependency unless a selected Bootstrap component requires it and native HTML cannot cover the behavior.
- No Svelte Bootstrap wrapper library.
- No requirement to share visual components between V1 and V2 pages.

## Decisions

1. Bootstrap is used directly.

   Add the `bootstrap` npm package and import the compiled Bootstrap CSS from the UI bootstrap path. Components should use ordinary Svelte markup and documented Bootstrap class names. Wrapper libraries such as `sveltestrap` stay out of scope because they add another component API for agents to learn and can hide the simple class contract.

2. V2 routes create the isolation boundary.

   The first Bootstrap implementation should add `#/v2/login` and `#/v2/finance`. These routes should use V2-specific route composition, and they should not import visual components whose main purpose is to carry the V1 custom design system. Sharing behavior-only helpers, API clients, auth stores, route guards, and formatting utilities is encouraged.

   For this repo shape, `#/v2/finance` must be explicit about what it reuses:

   - `App.svelte` or equivalent route wiring should treat `#/v2/finance` as its own protected finance-like boundary rather than falling through the generic authenticated shell.
   - The route should render inside a new Bootstrap-specific finance shell component instead of `FinanceShell.svelte`.
   - The Bootstrap shell may reuse the existing `FinanceShellState` provider or a small behavior-only extraction from it for tenant loading, selection persistence, and tenant-aware deep-link continuity.
   - The Bootstrap shell should own shell-level tenant selection, sign-out, theme controls, and compact finance-local navigation.
   - Because this change only pilots `#/v2/finance`, finance-local navigation may include handoff links back to canonical `#/finance/*` destinations for non-pilot browse and detail routes.
   - The generic authenticated `Nav` should not remain the primary chrome for `#/v2/finance`.

   The canonical `#/login` and `#/finance` routes remain available during the pilot. Promotion of V2 to canonical routes should be a later explicit task after visual review.

3. Custom CSS is exceptional in the pilot.

   The V2 pilot pages should not add `<style>` blocks, and should not use `style=` attributes for normal layout or visual styling. Allowed exceptions are:

   - app-shell containment that Bootstrap does not own
   - third-party widget sizing, such as chart containers
   - a small Bootstrap bridge or variable file
   - accessibility or browser-fix rules that cannot be expressed with Bootstrap classes

   Every exception must live in a shared stylesheet and have a short comment naming the gap it covers.

4. Bootstrap classes are the agent-facing design API.

   Implementation docs should name preferred patterns rather than broad taste guidance. Examples include `container-fluid`, `row`, `col-*`, `card`, `btn`, `form-control`, `alert`, `table`, `list-group`, `badge`, `nav`, `navbar`, spacing utilities, display utilities, and responsive utilities. Agents should prefer these before inventing local classes.

5. V2 login becomes the smallest proof of the rule.

   The `#/v2/login` page should use Bootstrap's standard centered form composition, labeled inputs, validation/error alert, submit button, and loading/disabled states. Behavior remains aligned with canonical login: successful login stores auth and routes to the remembered protected destination or `/data`; failure shows an inline error. Canonical `#/login` remains unchanged in this pilot.

6. V2 finance dashboard becomes the dense Bootstrap workbench proof.

   The `#/v2/finance` route should use Bootstrap grid, card, list, table, badge, form, and button patterns for the existing dashboard sections. The implementation should reuse existing finance dashboard/account/category/transaction/connection data sources, plus the shared finance tenant-workspace behavior, and keep honest loading, empty, and error states. The route should not reuse V1 shell visuals, but it should still preserve shell-level tenant context, sign-out, theme, and finance-local navigation responsibilities inside the new Bootstrap shell. Canonical `#/finance` remains unchanged in this pilot.

7. Existing custom design docs and smoke guidance must be reconciled during implementation.

   `DESIGN.md` currently declares the terminal-native custom system as the visual source of truth. The implementation must update it so the accepted V2 Bootstrap pilot direction is explicit and future agents do not face contradictory instructions. `ui-wireframe.md` must also describe the Bootstrap-based V2 login and V2 finance dashboard composition. `docs/manual-e2e/README.md` and `docs/manual-e2e/finance-ui-shell-smoke-e2e.md` must describe the parallel pilot smoke path and the fact that canonical routes remain unpromoted in this change.

8. Visual verification is part of each UI task.

   The implementation should start or reuse the local UI server, inspect `#/v2/login` and `#/v2/finance` in browser automation, capture desktop and mobile screenshots, check for console/network failures, and iterate until the pilot surfaces look coherent. The manual smoke docs should be updated early enough that these checks have a documented path to follow. This is task-level verification, not a detached final phase.

## Risks / Trade-offs

- Bootstrap will make the product look more conventional.
  - Mitigation: prefer consistent, shippable UI over bespoke uniqueness for this alpha surface.
- Bootstrap can still become class soup.
  - Mitigation: document a narrow set of approved patterns and enforce no page-local CSS in touched pilot files.
- Existing non-pilot pages may visually clash with Bootstrap pages.
  - Mitigation: isolate the first change under `#/v2/*` and accept temporary mismatch while measuring whether the rails improve delivery.
- The active finance-shell change may conflict with this one.
  - Mitigation: keep canonical `#/finance` stable and build the Bootstrap dashboard under `#/v2/finance` first.

## Migration Plan

1. Add Bootstrap and import global Bootstrap CSS.
2. Add or revise shared UI docs, AGENTS rules, and manual smoke docs for Bootstrap-first pilot work.
3. Add the V2 route composition and guarded/public route wiring for `#/v2/login` and `#/v2/finance`, including a Bootstrap-specific protected shell for `#/v2/finance` that reuses tenant/data behavior without reusing V1 finance shell visuals.
4. Build the V2 login page with Bootstrap markup.
5. Build the V2 finance dashboard with Bootstrap grid/card/form/list/table patterns using existing data and shared tenant-workspace behavior.
6. Keep canonical `#/login` and `#/finance` unchanged during the pilot.
7. Update `DESIGN.md`, `ui-wireframe.md`, and the relevant manual smoke docs to match the accepted Bootstrap pilot.
8. Run focused tests plus desktop/mobile visual smoke checks for the changed flows.

Because the project is early alpha, this change does not preserve a compatibility abstraction between V1 and V2 styling. It simply keeps V1 routes in place while V2 proves the new approach.

## Open Questions

- After visual review, should V2 be promoted by redirecting canonical routes, replacing canonical route components, or continuing as a separate route space for another slice?
