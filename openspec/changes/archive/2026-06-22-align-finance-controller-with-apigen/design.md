## Context

`apps/signal-foundry/internal/api/http/v1controllers/finance.go` currently implements the generated finance controller interface in two layers. The generated methods such as `ListFinanceAccounts` and `GetFinanceDashboard` only delegate to handwritten `http.Handler` helpers, and those helpers manually decode JSON bodies, read `req.PathValue(...)`, inspect `req.URL.Query()`, write JSON responses, and map controller errors outside the normal apigen path.

That divergence is already causing real contract drift. The OpenAPI file defines only path params for several finance routes, while the live controller also depends on query inputs such as:

- `includeHidden` on accounts, categories, tags, and transactions
- `accountId`, `source`, and `status` on transaction listing
- `preset`, `startDate`, and `endDate` on dashboard reads

Because those inputs are missing from `v1routes.yaml`, the generated finance params surface cannot represent the full live route contract. Fixing the controller cleanly therefore requires both OpenAPI/codegen alignment and controller refactoring.

## Goals / Non-Goals

**Goals:**

- Make `v1routes.yaml` the full source of truth for finance route inputs that the controller currently consumes.
- Refactor finance controller operations to use `builder.HandleWith` or `builder.HandleWithHTTP` directly, following the module's apigen convention.
- Preserve existing finance endpoint behavior: auth requirements, tenant scoping, camelCase JSON, current success payloads, current finance-specific domain error mappings, and current date/filter semantics.
- Remove redundant finance-only request/response plumbing once the generated handler pipeline owns parsing and serialization again.
- Expand controller tests so the refactor is protected by route-level evidence rather than a large blind rewrite.

**Non-Goals:**

- Do not change finance domain rules, tenant authorization rules, or service-layer business behavior.
- Do not redesign finance response models or add new finance routes.
- Do not change operator UI behavior or client contracts beyond documenting already-live finance inputs in OpenAPI.
- Do not broaden finance error semantics beyond preserving existing status behavior through the shared middleware path.

## Decisions

### 1. Update the finance OpenAPI contract before refactoring handler bodies

The first implementation step will be to update `apps/signal-foundry/internal/api/http/v1routes.yaml` so every finance route input currently used in `finance.go` exists in the generated params surface. That includes `includeHidden`, transaction list filters, and dashboard period inputs.

For dashboard period inputs, `startDate` and `endDate` will stay string query params representing `YYYY-MM-DD`, and the controller will keep explicit conversion into UTC `time.Time` values. This preserves current input semantics without depending on generator-specific date-only parsing behavior.

Why:

- The controller cannot meaningfully move to apigen builders if the generated params do not expose the route's real inputs.
- Updating the contract first turns the controller refactor into a typed migration instead of another round of handwritten query access.
- Keeping date-only dashboard inputs as strings preserves the current external contract exactly.

Alternatives considered:

- Refactoring the controller first while still reading raw `req.URL.Query()` was rejected because it would keep the same contract drift under a different shape.
- Converting dashboard query inputs to date-time params was rejected because the current endpoint accepts date-only values and the goal is alignment, not API redesign.

### 2. Collapse finance route implementations into the generated handler methods

Each finance operation method on `FinanceController` will become the real implementation entrypoint and return a builder-backed handler directly. Most routes should use `builder.HandleWith` because generated params will cover path, body, and query inputs; `HandleWithHTTP` should be reserved only for routes that still need raw request access after contract alignment.

The secondary methods such as `ListTenants()`, `CreateTenant()`, `GetDashboard()`, `ConfirmCSVImport()`, and the custom `wrap(...)` layer will be removed or absorbed into small focused helpers.

Why:

- This matches the controller pattern already used by auth, data, jobs, strategies, and evaluations.
- The generated handler path restores standard request parsing, validation, response serialization, and shared error handling.
- Removing the duplicate controller layer makes future finance route changes start in OpenAPI instead of in handwritten HTTP glue.

Alternatives considered:

- Keeping the current delegation pattern and only cleaning up helper names was rejected because it preserves the same parallel HTTP stack.
- Rewriting the finance controller around raw `http.Handler` only was rejected because it moves farther away from the module's documented route-generation approach.

### 3. Keep auth and actor resolution in context-based helpers, not request-specific wrappers

Finance handlers will continue to sit behind `AuthMiddleware`, but user identity lookup will happen inside builder-backed handlers from request context rather than through the current `wrap(...)` function. A small helper can extract the authenticated operator user ID from `httpapi.CallerIdentityFromContext` and return `app.NewErrUnauthorized("unauthorized")` when absent, matching current behavior.

Why:

- Other controllers already treat auth middleware plus context identity as the standard pattern.
- Finance service calls only need the actor user ID, not a bespoke transport wrapper.
- This keeps auth behavior consistent while eliminating one more handwritten routing layer.

Alternatives considered:

- Threading raw `*http.Request` through every finance service call path was rejected because it couples business parameter mapping to transport details.

### 4. Preserve narrow finance-specific error translation, but remove generic finance-only serialization helpers

The refactor will keep small domain-specific mapping helpers such as `mapCSVImportError` where finance errors need explicit translation. Generic helpers whose only job is replacing apigen behavior, such as manual JSON decoding, generic JSON writing, or the custom finance `wrap(...)` error-response path, should be deleted once no longer needed.

Why:

- Finance-specific domain error translation is valid controller logic.
- Manual decoding and response writing duplicate capabilities the generated handlers already provide.
- Shrinking finance-only HTTP plumbing reduces the chance of status/body behavior drifting from the rest of the app.

Alternatives considered:

- Keeping the generic helper stack for convenience was rejected because that is the source of the current inconsistency.

### 5. Protect the refactor with route-level controller tests that exercise generated params

`finance_test.go` already validates a broad portion of the finance HTTP surface through `server.NewTestRootHandler().RegisterFinanceRoutes(ctrl)`. The refactor should expand those tests around the currently fragile areas: `includeHidden`, transaction list filters, dashboard date parsing, auth failures, and CSV import error/status mappings.

Why:

- The refactor touches many endpoints but should preserve behavior.
- Route-level tests are the fastest way to catch mismatches between OpenAPI params, generated handlers, and service param mapping.
- They also prove that the OpenAPI additions are not documentation-only; the generated finance params must actually drive live route behavior.

Alternatives considered:

- Relying only on compile-time confidence after regeneration was rejected because query parsing and error/status behavior are runtime concerns.

## Risks / Trade-offs

- **[Spec and implementation drift during migration]** -> Update `v1routes.yaml` first, regenerate immediately, and keep controller edits in the same change.
- **[Large controller diff across many endpoints]** -> Leave mapping helpers untouched where possible and refactor operation bodies incrementally around existing service calls.
- **[Behavior regression in optional query parsing]** -> Add controller tests for `includeHidden`, transaction filters, and dashboard date validation before changing handler code.
- **[Error response differences when leaving the manual wrapper]** -> Cover current conflict/unauthorized/invalid-input cases in controller tests and preserve finance-specific translations explicitly.
- **[Generated-code churn obscuring review]** -> Limit handwritten changes to `v1routes.yaml`, `finance.go`, and tests, then treat regenerated route/model files as mechanical outputs.

## Migration Plan

No persistence or deployment migration is required. This is an application-contract and controller-shape refactor.

Implementation sequence:

1. Expand finance controller tests to capture the existing route behavior that currently depends on handwritten query/body parsing.
2. Update `apps/signal-foundry/internal/api/http/v1routes.yaml` so finance query inputs are explicitly declared.
3. Run `go generate ./internal/api/http/register.go` in `apps/signal-foundry` to regenerate finance handlers/models.
4. Refactor `internal/api/http/v1controllers/finance.go` so the generated finance controller methods are the real implementations.
5. Remove redundant finance-only wrapper/serialization helpers if no longer referenced.

Rollback:

- Revert the OpenAPI edits, regenerated outputs, controller refactor, and controller test updates together.
- No data rollback is required.

## Open Questions

None that block implementation.
