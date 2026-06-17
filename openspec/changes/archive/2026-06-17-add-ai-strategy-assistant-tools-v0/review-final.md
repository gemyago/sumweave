# Final Review: add-ai-strategy-assistant-tools-v0

## Round 1

- Scope: whole change review across `apps/signal-foundry/internal/strategyassistant`, runtime/profile wiring, bundled skills, chat UI, smoke coverage, and OpenSpec artifacts for `add-ai-strategy-assistant-tools-v0`
- Triggering input: whole-change implementation-finalizing review for the internal-alpha strategy assistant flow
- Findings:
  - `apps/signal-ui/src/components/ToolCallBlock.svelte:21-23` only derives quick links from flattened top-level `strategyId`, `version`, and `runId` fields (or `args.version`). The actual strategy assistant DTOs returned by this change are nested instead: strategy reads/saves return `version.{strategyId,version}` (`apps/signal-foundry/internal/strategyassistant/contracts.go:335-400`) and backtest runs return `run.runId` while the request uses `strategyVersion` rather than `version` (`apps/signal-foundry/internal/strategyassistant/contracts.go:403-462`). Because `streamState` forwards the raw tool response unchanged (`apps/signal-ui/src/lib/agentapi/streamState.ts:84-99`), real `sf_strategy_*` and `sf_evaluation_run_backtest` results will usually render without the promised strategy/evaluation route links. The current UI test coverage misses this because `apps/signal-ui/src/pages/Chat.test.ts:497-530` uses a synthetic flattened tool response shape that does not match the implemented DTOs.
- Verdict: needs fixes
- Cross-chunk assessment: data/strategy/evaluation handlers, runtime registration, seeded profile guidance, skills enablement, and smoke coverage otherwise fit together coherently and keep the intended no-live-trading / no-raw-SQL safety boundaries intact; the blocking gap is the chat UI’s DTO/link wiring across the runtime + tool + UI chunks.
- Artifact cleanup status: no stray journey/scratch/temp artifacts were found under the change directory or touched implementation areas; this final review artifact and the status update are the only new review outputs.
- Completion protocol status:
  - `make affected-lint-test` ✓ re-run during whole-change review
  - `go test ./apps/signal-foundry/internal -run TestStrategyAssistantSmoke -count=1` ✓ re-run during whole-change review
  - AGENTS.md updates: no changes needed
  - UI/UX manual smoke + visual assessment evidence: not present in the recorded artifacts for the UI chunk, so protocol evidence remains incomplete even though automated UI tests passed
  - Clean relevant git status gate before artifact update: satisfied
- Commit status: pending review-artifact commit

## Round 2

- Scope: follow-up re-review of the scoped chat UI quick-link fix in `apps/signal-ui/src/components/ToolCallBlock.svelte`, `apps/signal-ui/src/components/ToolCallBlock.test.ts`, and `apps/signal-ui/src/pages/Chat.test.ts`
- Triggering input: blocking whole-change finding from round 1 after the reported follow-up fix landed
- Findings:
  - None.
- Verdict: clean
- Verification notes: `ToolCallBlock.svelte` now reads strategy identifiers from nested `response.version.{strategyId,version}` payloads with flat-field fallback, and evaluation links from `response.run.runId` while using `args.strategyVersion` for backtest version fallback; that matches the implemented strategy/evaluation DTO contracts in `apps/signal-foundry/internal/strategyassistant/contracts.go`. `ToolCallBlock.test.ts` now exercises the real nested strategy-version response shape, and `Chat.test.ts` now streams a realistic `sf_evaluation_run_backtest` request/response payload through the chat SSE path, so the scoped regression coverage now matches production DTO structure instead of the earlier flattened synthetic shape.
- Artifact cleanup status: no stray journey/scratch/temp artifacts were found under the change directory or touched implementation areas; this final review artifact and the manager status update are the only follow-up review outputs.
- Completion protocol status:
  - `npx vitest run src/pages/Chat.test.ts src/components/ToolCallBlock.test.ts` ✓ re-run during scoped re-review
  - `make affected-lint-test` ✓ re-run during scoped re-review
  - AGENTS.md updates: no changes needed
  - UI/UX manual smoke + visual assessment evidence: reported present for `/#/chat` with mocked real-shape tool SSE ✓
  - Clean relevant git status gate before artifact update: satisfied by the follow-up fix scope; final clean state recorded in the follow-up commit
- Commit status: pending follow-up fix + review-artifact commit at review write time

## Round 3

- Scope: user review/correction feedback on tool visibility expectations and picker spacing in `apps/signal-ui/src/pages/Chat.svelte`
- Triggering input: user reported that the strategy assistant tool output still appears mismatched against the ticket expectation and noted a UI spacing issue between the profile picker and model picker
- Exact user quote: `Chat test results - I added provider and model - I selected strategy assistant profile - I asked model to list tools available Strategy related tools - the model reports a clear mismatch between what we have in ticket and what model shows in it's output

Small UI notes: no marging between Profile picker  and model picker`
- Findings:
  - None requiring additional feature work. The reported tool mismatch is not a ticket bug: this change explicitly accepted global internal-alpha tool registration while deferring profile-level tool filtering/authorization (`design.md` decisions 3 and risks/trade-offs; `specs/ai-strategy-assistant-tools/spec.md` scenario "Alpha registration defers fine-grained permissions"). The current runtime therefore intentionally does not scope the registered tool registry per profile in v0.
  - The picker spacing fix is clean. `apps/signal-ui/src/pages/Chat.svelte` now lets `.composer-bar-start` wrap with `gap: var(--space-8)`, which restores visible separation between the adjacent profile and model pickers without changing picker behavior.
- Verdict: clean
- Verification notes: the scoped implementation change is limited to the picker-row layout styles in `Chat.svelte`; no additional runtime/profile/tool-registry behavior changed, which is correct because the reported tool-list concern is covered by the approved v0 scope rather than a missing implementation.
- Artifact cleanup status: no stray journey/scratch/temp artifacts were found under the change directory or touched implementation area; this final review artifact and the manager status update are the only final follow-up outputs.
- Completion protocol status:
  - `npx nx test signal-ui --skipNxCache` ✓ reported by the comments-addressing sub-agent for the scoped fix
  - `npx nx lint signal-ui --skipNxCache` ✓ reported by the comments-addressing sub-agent for the scoped fix
  - `make affected-lint-test` ✓ reported by the comments-addressing sub-agent for the scoped fix
  - AGENTS.md updates: no changes needed
  - UI/UX manual smoke + visual assessment evidence: reported present for `/#/chat` with mocked real-shape tool SSE ✓
  - Clean relevant git status gate before artifact update: satisfied; only the scoped fix and review artifacts remain in the working tree
- Commit status: scoped fix committed as `2272948` (`docs: finalize strategy assistant user review fix`)
