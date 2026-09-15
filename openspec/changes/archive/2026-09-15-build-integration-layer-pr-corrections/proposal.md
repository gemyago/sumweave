## Why

PR #28 exposes concrete security, correctness, and UI-state defects in the
archived `build-integration-layer` implementation, and its required CI test is
currently red. Correct them before merge without rewriting the archived change
or broadening the approved Phase 1 integration surface.

## What Changes

- Preserve `swmd` credential transport safety across redirects and enforce its
  configured overall job-wait deadline on in-flight requests.
- Restore the exact presence-only `--token-stdin` configure contract.
- Map unexpected access-token validation failures to logged safe internal errors
  rather than treating operational failures as invalid credentials.
- Keep the Admin access-token history synchronized after rotation.
- Stabilize the independently failing Finance dashboard hierarchy test under the
  normal CI suite without changing dashboard behavior or global test thresholds.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `direct-finance-cli`: Clarify that credential-bearing redirects are
  destination-validated and that the overall wait deadline bounds each polling
  request.

## Impact

- Backend/direct client: `apps/sumweave/internal/swmdclient`,
  `apps/sumweave/cmd/swmd`, and their focused Go tests.
- Backend authentication: `apps/sumweave/internal/api/http/middleware` and its
  focused registered middleware tests.
- UI: `apps/sumweave-ui/src/pages/AdminAccessTokens.svelte`, its page test, and
  the unrelated failing `Finance.test.ts` CI test only.
- OpenSpec: one linked correction change. The archived change and canonical
  Phase 1 product scope remain intact.
