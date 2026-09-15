The ordered groups below are sequential correction chunks. Each implementation
task includes its focused tests and the repository-required completion checks;
do not start a later chunk before the prior chunk has passed its shallow review
and been committed. The archived `build-integration-layer` task list remains
historical and unchanged.

## 1. Direct-Client Transport And Wait Boundaries

- [x] 1.1 Enforce redirect destination validation and the overall `job wait` request deadline in `internal/swmdclient`; must follow TDD flow by first adding focused `httptest` cases that prove an HTTPS-to-HTTP redirect cannot receive an Authorization header, allowed safe requests still work, and a handler blocked on a polling request sees its request context cancelled at the configured wait timeout, then implement the narrow transport/deadline boundary while retaining normal TLS verification, finite request timeouts, 30-second lazy-job grace, and fake-clock polling coverage; run root `make postgres-bootstrap`, focused app tests, and `make affected-lint-test`.

## 2. Exact Stdin Configure Syntax

- [ ] 2.1 Restore the presence-only `swmd auth configure --token-stdin` parser contract in `cmd/swmd`; must follow TDD flow by first extending randomized command cases for bare-flag success and `--token-stdin=true`, `--token-stdin=false`, and arbitrary assigned-value rejection, each proving stdin is not read and existing or absent selected config remains unchanged on failure, then implement the smallest flag parsing change without defining `--token` or changing config precedence; run root `make postgres-bootstrap`, focused app tests, and `make affected-lint-test`.

## 3. Credential Validation Failure Mapping

- [ ] 3.1 Map unexpected access-token validation failures in credential middleware to the existing logged safe internal-error path while preserving indistinguishable `401 unauthorized` responses for `auth.ErrInvalidAccessToken`; must follow TDD flow by first adding Mockery-backed registered-middleware cases for the invalid sentinel and a wrapped store failure, including safe status/code/correlation behavior, handler non-execution, and correlation-context logging of the wrapped operational error, then implement only the typed error distinction and mapper integration; run root `make postgres-bootstrap`, focused app tests, and `make affected-lint-test`.

## 4. Rotated Token Metadata Refresh

- [ ] 4.1 Refresh the Admin token metadata list after successful rotation without discarding the separately held one-time replacement secret; must follow the UI test-first flow by adding faker-backed page coverage that the server list shows both the revoked original and replacement after rotation, then implement the narrow list refresh and run focused UI checks, the required UI visual review/manual changed-flow smoke, and `make affected-lint-test`.

## 5. Required CI Dashboard-Test Stabilization

- [ ] 5.1 Stabilize the failing Finance dashboard hierarchy test under the normal UI suite without changing dashboard behavior or global test thresholds; must follow test-first investigation by reproducing the suite-level readiness failure, preserving the existing hierarchy assertions, and using a deterministic rendered readiness condition or a narrowly justified test-local timeout only if necessary, then run the focused file, the complete UI test target without Nx cache, and `make affected-lint-test`.
