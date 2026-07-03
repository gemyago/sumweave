# Review Chunk: app-controller-job-cli-fixture-wiring

## Implementation round 2026-07-03

- Scope completed for tasks section 3 only.
- `openspec apply split-finance-root-service` was attempted from the repo root, but the local `openspec` CLI does not support `apply` (`unknown command 'apply'`), so implementation proceeded manually within the approved chunk scope.

### What changed

- Reworked `finance.New` / `finance.Finance` to expose the focused public services directly:
  - tenant
  - catalog
  - ledger
  - reporting
  - FX
  - CSV import
  - bank-link
  - bank-sync
- Added finance composition helpers so app wiring can build focused service collaborators from the module entrypoint, including bank-sync connector-backed provider mapping for app/runtime composition.
- Replaced app DI usage of direct `finance.NewService` wiring with `finance.New` plus focused service extractors in `apps/signal-foundry/internal/financeapp/register.go`.
- Split finance HTTP controller dependencies by focused service instead of the broad root finance service while keeping route behavior and payloads unchanged.
- Split finance job handler registration to depend on focused FX, CSV import, and bank-sync services.
- Updated jobs CLI priming to resolve focused finance services instead of the root finance service.
- Reworked finance fixture generation bootstrap to compose scenario seeding through focused services plus focused bank-link/bank-sync/FX services instead of passing the root finance service through the scenario path.

### Tests and checks

- Targeted compile/test passes run during implementation:
  - `go test ./finance ./apps/signal-foundry/internal/financeapp ./apps/signal-foundry/internal/api/http/v1controllers ./apps/signal-foundry/internal/api/http ./apps/signal-foundry/cmd/signal-foundry`
  - `make -C finance lint test`
  - `make -C apps/signal-foundry lint test`
- Final required repo check passed:
  - `make affected-lint-test`

### OpenSpec task updates

- Marked complete in `tasks.md`:
  - 3.1
  - 3.2
  - 3.3

### Notes for the next reviewer

- Section 4 has not been started.
- Root `finance.Service` still exists only as a remaining cleanup/removal concern for the final chunk.
- No AGENTS updates were needed; no workflow or architecture instructions changed.

### Artifact cleanup

- No ad-hoc repository artifacts were created.
- Durable output for this chunk is this file only.

## Finalization (2026-07-03)

- Result: `complete`
- Verdict: Scope-aligned implementation is present for section 3 and all listed task items are complete.
- Continue decision: continue
- Completion protocol status: `make affected-lint-test` passed; no lint/test blockers.
- Artifact cleanup status: clean for this chunk; no unclassified ad-hoc artifacts.
- Commit status: committed
- Affected follow-up chunks: `4.1` remains pending in `tasks.md` and manager ledger.
- Short status: Section 3 is complete and safe to continue to section 4.
