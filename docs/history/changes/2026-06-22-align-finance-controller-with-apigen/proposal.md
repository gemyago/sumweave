---
type: Historical record
title: "2026-06-22-align-finance-controller-with-apigen: proposal"
description: "Historical proposal record for 2026-06-22-align-finance-controller-with-apigen."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-22-align-finance-controller-with-apigen/proposal.md"
    title: "Original openspec/changes/archive/2026-06-22-align-finance-controller-with-apigen/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

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

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
