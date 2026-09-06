## Context

The archived `2026-09-06-add-phase0-transaction-classification` change established
the Phase 0 contracts now present in the main OpenSpec specifications. PR #13
review subsequently identified three accepted gaps in that implementation. The
archived artifacts remain immutable; this new focused change carries only the
corrections and deltas needed for those findings.

The backend gaps share the explicit classification path and must be corrected
first. The UI gap is independent and follows as a second ordered chunk. The
existing appdispatch retry/dead-letter model, finance authorization convention,
and optional-rule interaction remain authoritative.

## Goals / Non-Goals

**Goals:**

- Preserve ordinary retry behavior for transient category reads while retaining
  terminal outcomes for categories that are actually unavailable to a rule.
- Return the established empty `401` response for an explicit classification
  tenant denial.
- Treat each successful manual assignment's optional-rule offer as a distinct
  draft lifetime: a replacement offer receives fresh defaults, while rerenders
  of the same offer do not erase operator edits.
- Cover each corrected boundary with focused regressions and complete the PR
  comment, submission, and CI confirmation gates.

**Non-Goals:**

- Changing category availability rules, appdispatch policy, classification
  payloads, endpoint shapes, or job visibility.
- Changing tenant membership semantics or error response bodies.
- Redesigning the optional rule form or other transaction-editor behavior.
- Modifying the archived original change or addressing feedback beyond the three
  accepted PR #13 findings.

## Decisions

### Distinguish category absence from lookup failure at the service boundary

`ClassificationService` will recognize the persistence not-found sentinel as an
actually unavailable category. A nil result, hidden category, or category owned
by another tenant remains unavailable as well. Any other lookup error will be
wrapped with classification lookup context and returned as an ordinary error,
without a `TerminalFailureError`, so both observed and ordinary appdispatch
handlers retain their existing retry/dead-letter behavior. Partial counts and
already committed assignments remain unchanged.

Focused service tests will prove the not-found path is terminal and a generated
transient error remains discoverable with `errors.Is`, is not terminal, and
retains prior committed counts. Existing handler coverage remains the evidence
that ordinary service errors use transport retry semantics.

### Use the standard finance authorization mapping at the HTTP boundary

The explicit classification controller's error mapper will translate
`finance.ErrTenantAccessDenied` to `app.NewErrUnauthorized`, preserving the
underlying cause in the same style as other finance mappers. A registered-route
test will return that service error and assert the standard `401` and empty body.
Range validation behavior remains unchanged.

### Give each manual-rule offer an explicit draft lifetime

The form owners will expose a stable identity for the current offer and replace
that identity only after another successful category assignment creates another
offer. The form draft will initialize its match type, condition, and target
category from a new offer exactly once. State changes and rerenders belonging to
the same offer will preserve edits; replacement with a new offer will refresh all
defaults, including when its values happen to equal the prior offer.

Focused component coverage will first edit the current draft and prove same-offer
updates preserve it, then replace the offer and prove the latest defaults are
submitted. At least one owning transaction flow will cover a second assignment
while the first offer is visible, proving that replacement identity reaches the
form. This is a behavioral state correction only, so no visual layout change is
planned.

### Execute and gate two ordered correction chunks

1. **Backend corrections:** implement category lookup error classification and
   tenant-denial mapping with their focused finance and registered-route tests.
   Bootstrap PostgreSQL before backend tests, run focused checks, then run
   `make affected-lint-test`. Obtain a shallow chunk review and commit only after
   it is clean.
2. **UI correction:** implement offer-lifetime draft refresh and preservation
   with component and parent-flow tests. Run focused UI checks,
   `make affected-lint-test`, the required changed-flow browser smoke and visual
   assessment, and independent UI design review; align the Finance UI wireframe's
   optional-rule offer note with the corrected behavior. Obtain a shallow chunk
   review and commit only after it is clean.

After both chunks, a user-comment verification reviewer will check only the six
threads representing the three accepted findings against the implementation and
test evidence. Once clean, reply to and resolve every paired thread, push the
branch, and confirm required GitHub checks on the pushed SHA complete successfully
(intentional artifact-publication skips remain non-failures). Record each comment
ID, thread ID, URL, path, reply, resolution, commit SHA, push result, and check URL
in Crew Manager notes. Do not implement, commit, reply, resolve, or push before
the independent plan review is clean.

## Risks / Trade-offs

- **A wrapped sentinel could be mistaken for transient** -> Use `errors.Is` for
  the persistence not-found sentinel and test both wrapped ordinary and terminal
  classification explicitly.
- **Reactive synchronization could erase active edits** -> Key synchronization to
  offer identity rather than mutable draft values or unrelated parent rerenders.
- **A replacement can contain the same values as its predecessor** -> Make offer
  identity independent of description/category equality.
- **Thread pairs could receive inconsistent closure evidence** -> Verify one fix
  per finding, then post the same finding-specific evidence to both corresponding
  threads before resolving them.

## Migration Plan

No data migration or deployment ordering is required. Apply the backend commit
before the UI commit, complete focused user-comment verification, push both, and
wait for CI on the resulting SHA. A code rollback reverts the correction commits;
no persisted data shape changes.

## Open Questions

None.
