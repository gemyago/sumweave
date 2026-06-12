# Final Review

Whole-change review and user-correction history for `implement-data-layer-v0`.

## 2026-06-12 Whole-change final review

### Scope

- First final review: full review.
- Reviewed proposal, design, spec, tasks, implementation files, tests, manager artifacts, and repository verification evidence.

## Verdict

1. [runtime/data/database_store.go](/Users/jenya/projects/signal-foundry/runtime/data/database_store.go:45) and [runtime/data/database_store.go](/Users/jenya/projects/signal-foundry/runtime/data/database_store.go:181) do not implement the approved source-aware candle identity from [design.md](/Users/jenya/projects/signal-foundry/openspec/changes/implement-data-layer-v0/design.md:60). The `candles` uniqueness rule and conflict target only use `instrument_id`, `timeframe`, `start_at`, and `end_at`, so ingesting the same bar from a second source overwrites the first source's provenance and quality instead of preserving the source-aware record shape the design calls for.
2. [runtime/domain/types.go](/Users/jenya/projects/signal-foundry/runtime/domain/types.go:190), [runtime/data/service.go](/Users/jenya/projects/signal-foundry/runtime/data/service.go:522), and [runtime/data/database_store.go](/Users/jenya/projects/signal-foundry/runtime/data/database_store.go:310) disagree on valid trade identity when `provenance.recordID` is absent. The domain and service layers allow a blank record ID, but the trade uniqueness rule only keys on `instrument_id`, `provenance_source`, and `provenance_record_id`, so every trade from the same source and instrument with an empty record ID conflicts and overwrites prior rows instead of falling back to the event-timestamp-based identity described in [design.md](/Users/jenya/projects/signal-foundry/openspec/changes/implement-data-layer-v0/design.md:60).

## Affected Follow-up Chunks

- `gorm-persistence`
- `deterministic-query-replay`

## Completion Protocol Status

- Repository verification: pass — `make affected-lint-test` ran successfully on 2026-06-12, with Nx reporting cached passes for the affected lint/test targets.
- AGENTS.md update check: pass — no commands, workflows, or architecture guidance changed in the reviewed implementation.
- Whole-change completion gate: fail — blocking persistence-contract findings remain, so the change is not ready to advance to user review or submission.

## Artifact Cleanup Status

- clean

## Commit Status

- no commit created because the whole-change review is not clean; follow-up fixes are required before a final-review/status commit is appropriate

## Non-Blocking Notes

- Current persistence tests cover idempotency and replay with populated provenance IDs, but they do not cover blank trade `RecordID` handling or multi-source candles at the same bucket.

## 2026-06-12 Whole-change follow-up re-review

### Scope

- Follow-up re-review: lighter pass focused on the two prior blocking findings, obvious regressions, identity-case coverage, completion-protocol evidence, and artifact/status cleanup.

## Verdict

Clean. The prior blocking findings are resolved: candle persistence now keys same-bucket rows by source-aware identity in [runtime/data/database_store.go](/Users/jenya/projects/signal-foundry/runtime/data/database_store.go:46), [runtime/data/database_store.go](/Users/jenya/projects/signal-foundry/runtime/data/database_store.go:195), and [runtime/data/database_store.go](/Users/jenya/projects/signal-foundry/runtime/data/database_store.go:512), while trade persistence now falls back to event-time identity when `provenance.recordID` is blank in [runtime/data/database_store.go](/Users/jenya/projects/signal-foundry/runtime/data/database_store.go:67), [runtime/data/database_store.go](/Users/jenya/projects/signal-foundry/runtime/data/database_store.go:325), and [runtime/data/database_store.go](/Users/jenya/projects/signal-foundry/runtime/data/database_store.go:575). The lighter regression pass did not uncover any new obvious issues.

## Affected Follow-up Chunks

- none

## Completion Protocol Status

- Fixed identity coverage: pass — explicit SQLite-backed coverage now exercises multi-source candles in [runtime/data/database_store_test.go](/Users/jenya/projects/signal-foundry/runtime/data/database_store_test.go:393) and blank-trade-record-ID fallback behavior in [runtime/data/database_store_test.go](/Users/jenya/projects/signal-foundry/runtime/data/database_store_test.go:493); I also re-ran those targeted cases with `go test ./runtime/data -run 'TestDatabaseStore/(candle ingestion preserves distinct rows for the same bucket from different sources|trade ingestion falls back to event-time identity when provenance record ID is blank)' -count=1`.
- Repository verification: pass — `make affected-lint-test` succeeded on 2026-06-12 and Nx reported successful affected lint/test coverage across the repo.
- AGENTS.md update check: pass — no commands, workflows, or architecture guidance changed in the fix or this review update.
- Whole-change completion gate: pass — no blocking findings remain.

## Artifact Cleanup Status

- classified files — standard ignored tool outputs only: `runtime/.cover`, `apps/signal-foundry/.cover`, `tools/firecrawl/.cover`, `tools/workspacefs/.cover`, `tools/skills/.cover`, `apps/signal-ui/dist`, `apps/signal-ui/coverage`, `build/npm/dist`; `git status --short` is clean, so no disallowed ad-hoc repository artifacts remain

## Commit Status

- commit created for the clean follow-up re-review/status updates

## Non-Blocking Notes

- `make affected-lint-test` emitted the existing `./.envrc:19: scripts/nvm-auto: No such file or directory` warning before continuing successfully; it did not cause verification failures in this pass.

## 2026-06-12 User approval and release handoff

### Scope

- User-review completion.
- User confirmed the implemented change is approved.
- User explicitly requested archive followed by submission.

## Verdict

Clean. No further review corrections were requested by the user, so the implementation is approved to advance to archive and then submission.

## Affected Follow-up Chunks

- none

## Completion Protocol Status

- User review gate: pass — explicit user approval was given on 2026-06-12.

## Artifact Cleanup Status

- clean

## Commit Status

- pending archive and submission updates

## Non-Blocking Notes

- none
