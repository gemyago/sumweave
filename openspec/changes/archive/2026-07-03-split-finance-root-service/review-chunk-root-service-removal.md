# Review Chunk: root-service-removal

## Implementation round 2026-07-03

- Result: complete
- Phase: initial implementation phase

### What changed

- Removed the production root `finance.Service` facade, shared root options, and broad `serviceStore` dependency from `finance/`.
- Replaced `finance.New` composition with focused-service assembly through `focused_services_composition.go`.
- Added caller-audit coverage in `finance/root_service_caller_audit_test.go` to prove active non-test repo code no longer imports or constructs `finance.Service` / `finance.NewService` for product workflows.
- Rewired remaining finance/app tests to focused services where needed and restored non-delegation behavior coverage needed by finance coverage gates.
- Deleted the old production root delegating surface while keeping a test-only compatibility helper for existing behavior-focused test coverage.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/...`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`

### OpenSpec task updates

- Marked `tasks.md` item `4.1` complete.

### Artifact cleanup

- Removed obsolete production root-facade code.
- No ad-hoc repository artifacts added.

### Notes for next reviewer

- `finance.Service` / `finance.NewService` now exist only in test-only compatibility coverage, not in active non-test product code.
- `apps/signal-foundry/cmd/signal-foundry/finance_cmd_test.go` now reads focused tenant/reporting services directly instead of the removed root facade.

## Finalization round 2026-07-03

- Result: complete
- Verdict summary: root service facade removal is complete with focused-service composition in `finance.New` and a focused-services audit test proving product code no longer depends on `finance.Service` or `finance.NewService`.
- Continue decision: safe to continue.
- Completion protocol status: pass
- Artifact cleanup status: clean (no ad-hoc repository artifacts)
- Commit status: `dd73559`
- Affected follow-up chunks: none

### Checks run

- `make affected-lint-test`
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/...`
