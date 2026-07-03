## Implementation Round 1

- Scope: extend the manual API-only tenant-management e2e guide with get-by-id checks before archival and after archival, then run the documented local API flow.
- OpenSpec apply: attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply archive-finance-tenants`, but the installed CLI still does not provide `apply`; proceeded with direct file edits.
- Files updated:
  - `docs/manual-e2e/finance-tenants-management-e2e.md`
  - `docs/manual-e2e/README.md`
  - `openspec/changes/archive-finance-tenants/manager-status.md`
  - `openspec/changes/archive-finance-tenants/review-chunk-chunk-3.md`
- Manual flow run against local backend: pass.
  - Health check returned `200` from `http://127.0.0.1:4501/health`.
  - Documented auth flow succeeded using repo-root `.local-users`.
  - Tenant management flow results: create `200`, list-before `200`, get-before `200`, archive `204`, list-after `200`, get-after `404`.
- Fixes needed from run:
  - docs only — the previous guide did not include the now-required get-by-id checks, and the post-archive get-by-id result should be documented as `404`.
- OpenSpec task updates made: none; existing task coverage already matched the documentation-only follow-up.
- Artifact cleanup status: clean — used `/tmp` scratch response files only; no repository scratch artifacts added.
