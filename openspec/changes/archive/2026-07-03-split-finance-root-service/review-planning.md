# Planning Review

## Review round 2026-07-03

- Verdict: needs-follow-up
- Ready for implementation: no

### Findings

1. Missing explicit `finance.New` / `finance.Finance` contract work.
   - `proposal.md` and `specs/finance-management/spec.md` make focused services exposed from `finance.New` the new public contract.
   - `design.md` decision 1 also says `finance.New` should build and expose the focused services.
   - `tasks.md` never explicitly requires updating `finance.Finance`, `finance.New`, or module-level tests that prove `finance.New` exposes the new focused services and retires root-service usage. Task 3.1 only covers app DI, which leaves the core finance module contract underplanned.
   - Add explicit task coverage for the `finance.New`/`Finance` surface and matching finance module tests.

2. FX controller dependency split is omitted from design/task coverage.
   - `proposal.md` requires moving HTTP controller dependencies to the focused services they actually need.
   - The current finance controller still serves FX diagnostics and FX sync endpoints through the broad finance dependency.
   - `design.md` decision 4 and `tasks.md` item 3.2 enumerate tenant, catalog, ledger, reporting, import, bank-link, and bank-sync dependencies, but omit FX controller endpoints.
   - Add explicit planning coverage so FX controller routes also move off broad `finance.Service`, or the plan can still pass a root facade into active controller code and miss the stated requirement.

### Chunk plan

- Not approved yet. Re-plan after the blockers above are fixed.

### Artifact cleanup

- Clean. Present files are standard OpenSpec change artifacts plus manager/review files; no ad-hoc repository artifacts found.

### Commit status

- No commit created. The plan is not clean/ready for implementation, so the planning commit gate does not apply yet.

## Review round 2026-07-03 (re-review)

- Verdict: complete
- Ready for implementation: yes

### Findings

1. Previous planning blockers are addressed.
   - `proposal.md` now makes the `finance.New` / `finance.Finance` contract explicit, including focused-service exposure and contract-test coverage.
   - `design.md` decision 1 now defines `finance.New` building the `Finance` composition contract, and decision 4 explicitly includes FX diagnostics and FX sync controller endpoints in the dependency split.
   - `tasks.md` item 3.1 now requires failing module tests for the `finance.New` / `finance.Finance` contract before rewiring app DI, and item 3.2 explicitly moves FX controller routes onto the focused FX dependency.
   - The revised plan keeps scope bounded to the service-boundary split and preserves strict ordered chunking across sections 1 through 4.

### Chunk plan

- Approved as 4 sequential implementation chunks matching tasks sections 1 through 4.
- Keep the current strict order: core public service boundaries -> reporting/FX/import/bank-sync services -> module/app/controller/job/CLI/fixture wiring -> root service removal.
- Do not combine or reorder sections; section 4 remains the final cleanup/removal chunk.

### Artifact cleanup

- Clean. Present files are standard OpenSpec change artifacts plus manager/review files; no ad-hoc repository artifacts found.

### Commit status

- Approved planning artifacts should be committed together at the planning gate.

## Review round 2026-07-03 (user comment planning)

- Verdict: complete
- Ready for implementation: yes

### Findings

1. Nothing architectural requires `finance/service.go` to remain.
   - `proposal.md` already says the change should shrink or remove `finance/service.go`.
   - The remaining file contents are leftover declaration-only pieces: shared sentinel errors, tenant seed helpers, focused-service parameter structs, and transfer helpers.
   - The user comment is therefore a bounded cleanup follow-up, not a new design direction.

2. The user comment now has explicit task coverage as a separate follow-up chunk.
   - `tasks.md` adds section 5 for deleting `finance/service.go` after the already-complete root-service removal work.
   - The follow-up stays narrowly scoped to relocating the remaining declarations into focused files or narrowly named concept files, then deleting the shared bucket file.

### Chunk plan

- Approved as 1 sequential follow-up chunk after the already-complete sections 1 through 4.
- Ordered chunk:
  1. `service-go-file-decomposition` — relocate the remaining declarations out of `finance/service.go`, keep package behavior and exported names stable, and delete the file.

### Artifact cleanup

- Clean. Present files are standard OpenSpec change artifacts plus manager/review files; no ad-hoc repository artifacts found.

### Commit status

- No commit created in this planning-only comment round.
