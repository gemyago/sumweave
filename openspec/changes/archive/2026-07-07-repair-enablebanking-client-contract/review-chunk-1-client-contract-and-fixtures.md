# Review Chunk 1: client contract and fixtures

## Round 1 - 2026-07-07

- Phase: initial implementation phase
- Scope: client contract and fixtures only
- `openspec apply`:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change repair-enablebanking-client-contract --task 1.1 --task 1.2`
  - result: failed because the installed CLI reports `unknown command 'apply'`
- Implemented:
  - added docs-derived Enable Banking client fixtures under `finance/internal/enablebanking/client/testdata/enable_banking_docs/`
  - rewrote client tests around official `GET /aspsps`, `POST /auth`, `POST /sessions`, `GET /sessions/{session_id}`, account details, balances, and transactions response shapes and official query/request field names
  - added tests proving relevant response structs no longer expose `Raw` fields and that legacy alias payloads are ignored or rejected
  - replaced raw-map extraction in the covered client methods with typed JSON decoding plus typed normalization for compatibility fields still used by higher layers
  - removed raw-map fields from the covered client models and narrowed `CreateSessionRequest` to the documented `code` field
- Checks run:
  - `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/internal/enablebanking/client`
  - `direnv exec /Users/jenya/projects/signal-foundry golangci-lint run ./finance/internal/enablebanking/client/...`
- OpenSpec task updates:
  - marked `1.1` complete in `tasks.md`
  - marked `1.2` complete in `tasks.md`
- Artifact cleanup status:
  - clean; no ad-hoc repo artifacts created
- Blockers:
- none for chunk 1 scope

## Round 2 - 2026-07-07

### Verdict

needs-follow-up

### Findings

- Scope alignment: changed files are confined to `finance/internal/enablebanking/client` plus OpenSpec artifacts (`tasks.md`, `manager-status.md`, and this review file).
- Requested task 1.1 and 1.2 are both marked complete in `tasks.md`, with fixtures/tests added and raw-map typed contract cleanup present.
- Client implementation now decodes documented field names for GET /aspsps, POST /auth, POST /sessions, GET /sessions/{session_id}, account details, balances, and transactions.
- Model changes removed schema-level `Raw` exposures from the touched response structures.
- Tests were run again here: `go test ./finance/internal/enablebanking/client` and pass.
- `golangci-lint run ./finance/internal/enablebanking/client/...` could not be executed because `golangci-lint` is not installed in this environment.
- `openspec apply` was not available (`unknown command 'apply'`), so no CLI-based apply artifacts exist from this run.
- No obvious functional regressions are visible in the touched package from a shallow review, but lint verification and commit are still pending.
- Targeted connector-level validation exposed expected regressions from this contract shift:
  - `go test ./finance/internal/enablebanking -run 'TestConnector|TestBankConnectionService|TestWindowSyncExecutorRealConnectorComposition'`
  - fails in StartLink/FinishLink/session fetch flow due legacy fixture/expectation mismatches against the new typed contract.

### Completion protocol status

- OpenSpec implementation tasks for chunk 1: complete in `tasks.md`.
- Local verification: package-level client tests pass; connector/integration tests fail until chunk 2-3 alignment work is completed.
- make affected-lint-test: fails in this environment because finance package tests currently fail against the shifted typed-session/auth contract.

### Artifact cleanup status

- Clean for this chunk scope: no stray repo artifacts discovered beyond expected test fixtures and OpenSpec review/status files.

### Commit status

- No commit yet; `git status` shows pending chunk 1 and review/plan artifacts.
- Commit gate not satisfied in this environment because a commit has not been created yet.

### Continue decision

- Conditional continue to chunk 2 after lint validation environment/tooling issue is resolved and chunk-specific files are committed as required by manager flow.

### Short status

- Verdict: needs-follow-up
- Continue: conditional
- Completion protocol: partial
- Artifact cleanup: clean
- Commit: pending
- Follow-up chunks: `2-typed-request-sending-and-app-wiring`, `3-connector-alignment`

## Round 3 - 2026-07-07

### Verdict

complete

### Findings

- Scope alignment: changed files for this chunk include `finance/internal/enablebanking/client` and expected OpenSpec artifacts.
- Requested task `1.1` and `1.2` were completed in `tasks.md` with documented fixtures and typed models/tests.
- Response models and test fixtures now target documented Enable Banking field names and avoid exposing schema-level `Raw` map fields.
- Contract-aligned JSON fixtures were added under `finance/internal/enablebanking/client/testdata/enable_banking_docs`.

### Completion protocol status

- Chunk 1 tasks are complete in `tasks.md`.
- Focused package tests for the updated client pass, and `make affected-lint-test` also passes after follow-up typed fixture updates in chunks 2 and 3 follow-up work.

### Artifact cleanup status

- Clean for this chunk scope: no stray repo artifacts discovered beyond expected test fixtures and OpenSpec artifacts.

### Commit status

- No commit yet; chunk 1 artifacts and related files are pending commit.

### Continue decision

- Proceed to chunk 3.

### Short status

- Verdict: complete
- Continue: proceed
- Completion protocol: passed (as far as chunk 1 scope is concerned)
- Artifact cleanup: clean
- Commit: pending
- Follow-up chunks: `3-connector-alignment`
