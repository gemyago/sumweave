---
type: Historical record
title: "2026-08-14-adopt-typed-provider-source-snapshots: review chunk protected finance api"
description: "Historical review chunk protected finance api record for 2026-08-14-adopt-typed-provider-source-snapshots."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-08-14-adopt-typed-provider-source-snapshots/review-chunk-protected-finance-api.md"
    title: "Original openspec/changes/archive/2026-08-14-adopt-typed-provider-source-snapshots/review-chunk-protected-finance-api.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

# Chunk Review: Protected Finance API

## Initial State

- Scope: tasks 4.1-4.2.
- Status: predecessor chunk complete; implementation started after the connector
  mapping/sync coordination review approval.

## Implementation

- Replaced the account and transaction OpenAPI `/evidence` paths, operation
  names, parameter names, and response contracts with
  `/provider-snapshots`. Metadata now has `id`, `kind`, `providerObjectId`, and
  `capturedAt`; detail responses use optional `data`.
- Regenerated apigen routes and validation/models, removed the obsolete generated
  evidence contract files, and implemented only the new snapshot controllers.
  No compatibility routes or aliases remain.
- The controllers depend on a focused provider-snapshot service. Sumweave HTTP
  composition now receives the `finance.Finance.ProviderSnapshotService`, whose
  list and detail methods enforce tenant membership/finance-object ownership and
  sanitize source documents at the service boundary.
- Registered-route tests cover account and transaction metadata/detail response
  shapes, no document in metadata responses, the `data` source-document shape,
  absent legacy routes, tenant/cross-tenant denial, missing snapshots, and
  ordinary service failures. Finance service tests retain the defense-in-depth
  document-sanitization coverage.

## Result

- Status: complete; tasks 4.1-4.2 are checked and the ordered UI chunk (tasks
  5.1-5.2) may begin.
- Commit: none; user explicitly requested no commit.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
