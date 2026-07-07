# Review Chunk 2: typed request sending and app wiring

## Round 1 - 2026-07-07

- Phase: initial implementation phase
- Scope: typed request sending and app wiring only
- `openspec apply`:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change repair-enablebanking-client-contract --task 2.1 --task 2.2`
  - result: failed because the installed CLI reports `unknown command 'apply'`
- Implemented:
  - replaced the Enable Banking client's raw-style transport helper usage with generic typed JSON send helpers that carry typed request bodies, typed decoded responses, JWT/bearer authorization, JSON headers, standard status handling, and optional response-body capture for later evidence work
  - updated all normal Enable Banking client operations in chunk scope to use the typed sender
  - wired `apps/signal-foundry/internal/financeapp` to require and use an app-created `httpclient.ClientFactory` client instead of `http.DefaultClient`
  - added app DI coverage proving Enable Banking requests use the factory-created transport and updated financeapp Enable Banking fixtures to the corrected chunk 1 contract where needed for this chunk to run
- Checks run:
  - `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/internal/enablebanking/client`
  - `direnv exec /Users/jenya/projects/signal-foundry ./bin/golangci-lint run ./finance/internal/enablebanking/client/...`
  - `direnv exec /Users/jenya/projects/signal-foundry go test ./apps/signal-foundry/internal/financeapp`
  - `direnv exec /Users/jenya/projects/signal-foundry ../../bin/golangci-lint run ./internal/financeapp/...` (from `apps/signal-foundry`)
- OpenSpec task updates:
  - marked `2.1` complete in `tasks.md`
  - marked `2.2` complete in `tasks.md`
- Artifact cleanup status:
 - clean; no ad-hoc repo artifacts created
- Blockers:
 - none for chunk 2 scope

## Round 2 - 2026-07-07

- Phase: finalization review
- Scope: typed request sending and app wiring only
- `openspec apply`:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change repair-enablebanking-client-contract --task 2.1 --task 2.2`
  - result: failed because the installed CLI reports `unknown command 'apply'`

- Scope alignment:
  - changed files are confined to `finance/internal/enablebanking/client`, `apps/signal-foundry/internal/financeapp`, and expected OpenSpec artifacts.
  - chunk2 OpenSpec tasks `2.1` and `2.2` are marked complete in `tasks.md`.

- Functional checks:
  - typed sender (`sendJSON`) replaced raw helper usage in the touched client methods.
  - finance module now receives app-created HTTP client from `httpclient.ClientFactory`.
  - app DI/transport tests were extended to assert header and client usage behavior.
  - no obvious issues introduced in this scope-specific implementation pass.

- Completion protocol check:
  - `make affected-lint-test` was re-run.
  - result: failed in `finance:test` due pre-existing/connector-facing contract fallout:
    - `finance` and `finance/internal/enablebanking` tests expecting legacy auth/session fixtures fail after chunk1/2 typed model changes.
    - `finance/internal/providers/window_sync_executor_real_test` fails on typed session decode shape differences.
  - because repository protocol requires successful `make affected-lint-test`, completion gate is not satisfied yet.

- Artifact cleanup status:
  - clean; no unclassified ad-hoc repository artifacts found in this chunk.

- Blockers / follow-up:
  - connector tests and finance window-sync tests need chunk 3 model/normalization alignment and fixture updates.

- Verdict summary:
  - Verdict: needs-follow-up
  - Continue decision: conditional
  - Completion protocol status: failed (gate not satisfied)
  - Artifact cleanup status: clean
  - Commit status: none
  - Follow-up chunks: `3-connector-alignment`

## Round 3 - 2026-07-07

- Phase: finalization review
- Scope: typed request sending and app wiring only
- `openspec apply`:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change repair-enablebanking-client-contract --task 2.1 --task 2.2`
  - result: failed because the installed CLI reports `unknown command 'apply'`

- Completion and scope checks:
  - `make affected-lint-test` was re-run across affected targets and now passes.
  - changed files remain constrained to `finance/internal/enablebanking/client`, `apps/signal-foundry/internal/financeapp`, finance/service-level and provider tests adjusted for strict typed fixture shapes, and expected OpenSpec artifacts.
  - chunk 2 OpenSpec tasks `2.1` and `2.2` are marked complete in `tasks.md`.

- Functional checks:
  - typed sender (`sendJSON`) is used for covered Enable Banking client methods.
  - finance uses app-created HTTP client from `httpclient.ClientFactory`.
  - app DI/transport tests continue to pass for factory-backed transport coverage.

- Completion protocol check:
  - `make affected-lint-test` was re-run and passed.

- Artifact cleanup status:
  - clean; no unclassified ad-hoc repository artifacts found.

- Follow-up:
  - remaining connector normalization and evidence-boundary work remains in chunk 3.

- Verdict summary:
  - Verdict: complete
  - Continue decision: proceed
  - Completion protocol status: passed
  - Artifact cleanup status: clean
  - Commit status: none
  - Follow-up chunks: `3-connector-alignment`
