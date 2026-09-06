The numbered groups are the two ordered correction chunks. Each group must use
`openspec apply`, mark only completed tasks, receive the OpenSpec overlay's
shallow review, and be committed before the next group starts. The design-level
user-comment verification, thread response/resolution, push, and post-push CI
gates follow both chunks and are not separate checklist tasks.

## 1. Backend Classification Corrections

- [x] 1.1 Correct classification category lookup and explicit tenant-denial boundaries; must follow TDD flow by first adding focused randomized Mockery-backed finance service cases proving persistence not-found, nil, hidden, and cross-tenant category states remain terminal while another wrapped category lookup error remains ordinary, preserves `errors.Is`, prior committed assignments, and available counts, then adding a registered-route controller case proving `ErrTenantAccessDenied` returns the standard empty `401` response without changing range failures. Implement the smallest service and error-mapping changes, confirm existing observed and ordinary handler tests still demonstrate retry/dead-letter treatment for ordinary service errors, run `make postgres-bootstrap` before backend tests, run focused finance and app controller/handler tests and lints, then run `make affected-lint-test`; assess `AGENTS.md` and record no change unless an established command, workflow, or architecture rule changed.

## 2. Manual-Rule Offer Draft Correction

- [x] 2.1 Correct the optional rule form's offer lifetime so a replacement offer refreshes defaults without erasing edits for the same offer; must follow TDD flow by first adding faker-backed component coverage that edits match type, condition, and category, proves same-offer rerenders preserve the draft, replaces the offer (including an equal-valued replacement), and proves submission uses the replacement defaults, plus a focused owning transaction flow that creates a second offer while the first remains visible. Implement explicit replacement identity across the form and both owners without changing layout or unrelated editor behavior, update `apps/sumweave-ui/ui-wireframe.md` with the replacement-versus-same-offer draft behavior, run focused UI component and parent tests and lint, then run `make affected-lint-test`; complete the changed-flow browser smoke, responsive visual assessment, and independent UI design-review flow described in the design, resolving concrete findings before shallow review, and assess `AGENTS.md` with no update unless an established command, workflow, or architecture rule changed.
