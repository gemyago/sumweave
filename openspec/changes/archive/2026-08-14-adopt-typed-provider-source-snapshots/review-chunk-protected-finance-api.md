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
