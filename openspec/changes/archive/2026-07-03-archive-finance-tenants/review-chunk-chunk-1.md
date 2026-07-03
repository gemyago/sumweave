# Chunk Review: chunk-1

Implementation and review history for chunk `chunk-1`.

## 2026-07-03 Implementation

Verdict: complete for chunk scope.

### Implemented

- Added finance tenant soft-archive state on the domain and persistence model, including active-list filtering and archive timestamp persistence without deleting tenant-owned records.
- Added tenant archive service behavior so current members can archive a tenant and archived tenants fail active tenant access checks.
- Tightened both finance access guard paths so archived tenants are rejected as active workspace context.
- Added `POST /api/v1/finance/tenants/{tenantId}/archive`, regenerated apigen route artifacts, and mapped the archive response onto the existing camelCase finance tenant summary surface with optional `archivedAt`.
- Added focused persistence, service, access-guard, and registered-route controller tests for archive behavior.

### Checks

- `openspec apply --change archive-finance-tenants --task 1` *(fails in current CLI: `unknown command 'apply'`)*
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance ./finance/persistence ./apps/signal-foundry/internal/api/http/v1controllers`
- `direnv exec /Users/jenya/projects/signal-foundry make clean-lint-cache && direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`

### Recheck (2026-07-03)

- Re-ran `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` after current edits.
- Result: passed (nx reports lint/test pass for affected modules).
- `apps/signal-foundry` finance controller and `finance` package tests include the new archive coverage.

### OpenSpec updates

- Marked tasks `1.1` and `1.2` complete in `tasks.md`.
- Updated `manager-status.md` to mark `chunk-1` complete.

## Completion Protocol Status

- Root coding protocol: pass after `make affected-lint-test`.
- `finance/AGENTS.md` protocol: pass.
- `apps/signal-foundry/AGENTS.md` protocol: pass, including route regeneration after OpenAPI edits.
- `AGENTS.md` update: not needed.
- `openspec apply` note: the installed CLI does not expose an `apply` subcommand, so the approved chunk was implemented directly while keeping OpenSpec task artifacts updated.

## Artifact Cleanup Status

- Clean with respect to artifact type: only standard OpenSpec artifacts were added or updated.

## Commit Status

- Commit series: `2cf0acb`, `a14f050`, `85b5da3`.
