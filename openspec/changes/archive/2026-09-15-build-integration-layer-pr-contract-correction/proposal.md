## Why

PR #28 has one new unresolved must-fix review thread at
`openspec/specs/direct-finance-cli/spec.md:83`. Archiving the prior
`build-integration-layer-pr-corrections` delta replaced, rather than extended,
the affected requirements' scenario lists. The canonical direct-client spec
therefore no longer states already implemented and approved transport and
observed-job-wait behavior.

## What Changes

- Restore the retained non-loopback HTTP rejection and HTTPS verification/
  finite-timeout scenarios alongside the redirect-destination scenario.
- Restore the retained initial-404 grace, late-404 terminal, and terminal-job
  scenarios alongside the in-flight overall-deadline scenario.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `direct-finance-cli`: Correct the canonical requirement deltas so the prior
  Phase 1 scenarios and the first correction round's additive scenarios coexist.

## Impact

- OpenSpec only: one active change and the canonical `direct-finance-cli` spec
  when this change is later archived.
- No Go, UI, API, schema, runtime, documentation, archive, or GitHub changes.
