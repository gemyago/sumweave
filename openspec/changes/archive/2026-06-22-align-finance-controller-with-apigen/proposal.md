## Why

The finance controller is the main backend HTTP surface in `apps/signal-foundry` that still bypasses the generated apigen handler flow and instead relies on custom wrappers, manual JSON decoding, and handwritten query parsing. That divergence is already hiding contract drift, because some live finance query parameters such as `includeHidden`, transaction filters, and dashboard period inputs exist in controller code but are missing from the OpenAPI-generated params surface.

## What Changes

- Align finance HTTP operations with the same apigen controller approach already used by auth, data, jobs, strategies, and evaluations controllers.
- Update the finance OpenAPI route contract so generated finance params cover the query inputs that are currently parsed manually in `finance.go`.
- Refactor finance controller actions to use `builder.HandleWith` or `builder.HandleWithHTTP` as the primary route implementation path instead of delegating to handwritten `http.Handler` helpers.
- Preserve existing finance API behavior, tenant scoping, auth requirements, and camelCase JSON responses while removing redundant controller-only request/response plumbing.
- Keep finance-specific error mapping only where the finance domain requires it, while routing normal request parsing, serialization, and status handling back through the shared apigen pipeline.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: Strengthen the protected finance HTTP API contract so finance routes are described fully in OpenAPI and implemented through the generated apigen handler pipeline instead of a parallel handwritten controller layer.

## Impact

- Affects `apps/signal-foundry/internal/api/http/v1routes.yaml` plus generated finance route params and handlers.
- Affects `apps/signal-foundry/internal/api/http/v1controllers/finance.go` and finance controller tests.
- May remove or shrink finance-only manual HTTP helpers once finance routes no longer depend on handwritten JSON decoding and custom wrapper flow.
- Updates the `finance-management` OpenSpec delta so the finance backend contract explicitly covers apigen-driven request parsing, auth context use, and query parameter completeness.
