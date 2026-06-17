# Manager Status

## Current State

- Phase: complete
- Task reference: GitHub issue https://github.com/gemyago/signal-foundry/issues/22
- Change slug: add-durable-historical-ingestion-jobs
- Last updated: 2026-06-17 submission completed with PR #23 on branch `feat/jobs-iteration-1`

## Workflow Board

- Planning: approved
- Implementation: complete (`jobs-foundation` finalized, `jobs-http-api` finalized, `jobs-agent-tools-skills` finalized, `jobs-ui-workspace` finalized, `jobs-integration-docs` finalized)
- Final review: clean
- User review/correction: complete
- Archive: complete
- Submission: complete

## Standard Artifacts

- Proposal: `proposal.md`
- Design: `design.md`
- Tasks/chunk plan: `tasks.md`
- Planning review/status: `review-planning.md`
- Final review/status: `review-final.md`
- Spec deltas:
  - `specs/durable-ingestion-jobs/spec.md`
  - `specs/historical-data-backfill/spec.md`
  - `specs/ai-strategy-assistant-tools/spec.md`
  - `specs/historical-data-browser/spec.md`

## Chunk Ledger

| Chunk | Scope | Status | Review file | Commit |
| --- | --- | --- | --- | --- |
| `jobs-foundation` | durable store/service/worker/backfill executor | finalized | `review-chunk-jobs-foundation.md` | see git history |
| `jobs-http-api` | app OpenAPI/routes/controllers/integration | finalized | `review-chunk-jobs-http-api.md` | see git history |
| `jobs-agent-tools-skills` | assistant job tools and workflow skill | finalized | `review-chunk-jobs-agent-tools-skills.md` | see git history |
| `jobs-ui-workspace` | Jobs workspace and Data-page entry | finalized | `review-chunk-jobs-ui-workspace.md` | see git history |
| `jobs-integration-docs` | product-flow coverage and docs/status | finalized | `review-chunk-jobs-integration-docs.md` | see git history |

## Agent Runs

| Phase | Agent | Scope | Status | Notes |
| --- | --- | --- | --- | --- |
| planning | openspec-planning | issue 22 | ready for review | artifacts revised for stale-running recovery and idempotency conflict semantics; implementation not started |
| implementation-finalizing | openspec chunk-finalizing | `jobs-foundation` | clean | reviewed jobs foundation chunk, re-ran `make affected-lint-test`, prepared chunk commit |
| implementation | openspec-implementation | `jobs-http-api` | complete | added protected `/api/v1/jobs` app routes/controllers plus integration coverage for API-created historical backfill jobs |
| implementation-finalizing | openspec chunk-finalizing | `jobs-http-api` | clean | reviewed HTTP API chunk, re-ran `make affected-lint-test` plus focused HTTP tests, and committed chunk changes |
| implementation | openspec-implementation | `jobs-agent-tools-skills` | complete | added assistant historical backfill start/list/get tools plus bundled `historical-data-jobs` workflow skill |
| implementation-finalizing | openspec chunk-finalizing | `jobs-agent-tools-skills` | clean | reviewed assistant jobs tools/skill chunk, re-ran `make affected-lint-test` plus focused strategy assistant tests, and prepared chunk commit |
| implementation | openspec-implementation | `jobs-ui-workspace` | complete | added jobs UI client/routes/pages, explicit Data-page backfill action, focused Vitest coverage, and updated wireframe behavior docs |
| implementation-finalizing | openspec chunk-finalizing | `jobs-ui-workspace` | clean | reviewed protected jobs routes/pages and explicit Data-page backfill flow, re-ran `make affected-lint-test` plus UI lint/test, and prepared chunk commit |
| implementation | openspec-implementation | `jobs-integration-docs` | complete | added local durable-jobs product-flow smoke coverage, updated manual e2e notes, and marked chunk task complete |
| implementation-finalizing | openspec chunk-finalizing | `jobs-integration-docs` | clean | reviewed local product-flow smoke/docs chunk, re-ran `make affected-lint-test` plus focused internal smoke tests, and committed chunk changes |
| implementation-finalizing | openspec implementation-finalizing | whole change | complete | reviewed cross-chunk durable jobs behavior, re-ran `make affected-lint-test` plus direct smoke coverage, updated final review/status artifacts, and advanced the change to user review/correction |
| archive | openspec archive | add-durable-historical-ingestion-jobs | complete | archived as `2026-06-17-add-durable-historical-ingestion-jobs` and promoted spec changes into `openspec/specs/*` |
| submission | manager | submission start | complete | prepared archive commit, pushed branch `feat/jobs-iteration-1`, and opened PR #23 |
| submission | gh pr create | PR #23 | complete | opened https://github.com/gemyago/signal-foundry/pull/23 against main from `feat/jobs-iteration-1` |

## Notes / Remaining Risks

- Community-manager reference repository was not accessible unauthenticated during planning (GitHub returned 404); design documents a DB-backed v0 worker with Watermill optional only as a wake mechanism.
- Stale-running recovery is now specified: startup requeues stale running jobs below max attempts and fails them at/above the cap with bounded details.
