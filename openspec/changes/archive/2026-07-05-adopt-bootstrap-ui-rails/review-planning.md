# Planning Review

## Review round 1 — 2026-07-05

- Verdict: blocked — stay in planning
- Ready for implementation: no

### Summary

The change direction is coherent, but the plan is not implementation-ready yet.

### Findings

1. V2 finance route composition is underspecified for this repo shape.
   - The current app routes, auth shell, and tenant-aware finance behavior are wired through `App.svelte` and `FinanceShell.svelte`.
   - The proposal/design say `#/v2/finance` should avoid importing V1 visual components, but they do not decide what replaces the current finance shell responsibilities for the pilot.
   - The plan must explicitly state whether `#/v2/finance` gets:
     - a Bootstrap-specific shell,
     - behavior-only reuse of the existing finance shell state/provider,
     - shell-level tenant selection,
     - sign-out/theme controls,
     - finance-local navigation links.
   - Without that decision, implementation can easily drift into either reusing V1 visual chrome against the design intent or dropping required finance route behavior.

2. Related documentation work is scattered across non-consecutive parent tasks.
   - Task `1.2` updates route/docs/rules for the V2 pilot.
   - Task `4.1` revisits the same route-status documentation later.
   - This violates the review rule against scattering related work across non-consecutive parent tasks.

3. Task `4.1` conflicts with the proposal/design deferral on promotion.
   - The proposal and design explicitly defer the canonical-route promotion decision to later work.
   - Task `4.1` says to “document the post-pilot promotion choice” while also saying no promotion happens here.
   - That should be corrected by either:
     - removing `4.1`, or
     - folding a simpler “pilot remains parallel; no promotion in this change” documentation check into `1.2`.

4. Proposal/tasks drift on manual e2e documentation.
   - The proposal impact names relevant manual e2e guide sections as affected docs.
   - The tasks include visual verification work, but no task updates any manual e2e guide.
   - Either add the documentation task or narrow the proposal impact statement.

### Required corrections

- Keep this change in planning.
- Update proposal/design/tasks to make V2 finance shell/state reuse explicit.
- Remove or fold task `4.1` into the earlier documentation/routing foundation work.
- Reconcile the manual e2e documentation commitment between proposal and tasks.

### Corrected strict ordered chunk plan

1. Bootstrap rails foundation: `1.1` + `1.2`, with the “pilot stays parallel / no promotion in this change” documentation folded here.
2. V2 login pilot: `2.1` + `2.2`.
3. V2 finance dashboard pilot: `3.1` + `3.2` + `3.3`, after the shell/state decision is made explicit in the plan.

### Artifact cleanup

- Standard planning artifacts are present.
- `README.md` exists in the change directory; classify it as a required manager/change artifact or remove it if it is ad-hoc.

### Commit gate

- No commit created because the planning gate is not clean.
