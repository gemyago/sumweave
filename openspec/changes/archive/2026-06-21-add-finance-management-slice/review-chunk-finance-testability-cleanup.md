# Chunk Review — finance-testability-cleanup

## Round 1

- Trigger: follow-up cleanup set review for chunk 9.2
- Verdict: not review-clean yet
- Scope fit: yes, changes stay within controller/job testability and coverage cleanup concerns
- What is correct:
  - controller test doubles were moved onto project-standard mockery output via `apps/signal-foundry/.mockery.yaml` and regenerated `internal/api/http/v1controllers/mocks_test.go`
  - focused controller tests now use those generated mocks for finance and jobs services
- Blocking issue:
  - the coverage-ignore cleanup is incomplete; finance package exclusions still remain in `finance/imports.go`, `finance/persistence/store.go`, and `finance/fixtures/realistic.go`, so the chunk does not satisfy the stated "no coverage-ignore style exclusions" requirement
- Verification:
  - `go test ./finance/...` passed
  - app-scope focused tests could not be completed cleanly because chunk 9.1's generated-validator compile failure still blocks the app packages
- Completion protocol: not satisfied
- Commit status: no commit yet is acceptable because the chunk is not review-clean
