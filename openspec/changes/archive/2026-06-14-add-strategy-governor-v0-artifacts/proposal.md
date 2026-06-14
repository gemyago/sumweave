## Why

The deterministic strategy and governor slices currently evaluate in-memory requests, but the MVP still lacks immutable product artifacts for approved strategy definitions and paper-only governor policies. This change adds the smallest runtime artifact layer needed to persist, identify, and reuse those definitions without adding backend, UI, or live-trading surfaces.

## What Changes

- Add immutable, versioned `StrategyArtifact` storage for strict v0 strategy definitions.
- Add a strict minimal moving-average-crossover Strategy DSL with validation, typed canonicalization, and mapping into the existing `strategy.EvaluateRequest` boundary.
- Add immutable, versioned, paper-only `GovernorPolicy` artifact storage with canonical JSON hashing and active-policy lookup.
- Add duplicate handling based on canonical artifact hashes so repeated creates of the same canonical artifact are idempotent.
- Add focused SQLite-backed persistence tests for artifact creation, retrieval, listing, duplicate handling, active governor policy lookup, and immutability guarantees.
- Exclude backend routes, OpenAPI changes, UI screens, live execution policy support, arbitrary strategy code, strategy output persistence, and governor decision/audit trace persistence.

## Capabilities

### New Capabilities
- `strategy-artifacts`: Defines immutable, versioned persisted strategy artifacts with canonical JSON bytes, canonical hashes, create/get/list behavior, duplicate handling, and no backend/API surface.
- `strategy-dsl`: Defines the strict v0 moving-average-crossover Strategy DSL, validation, canonicalization, arbitrary-code-field rejection, and mapping to existing strategy evaluation requests.
- `governor-policy-artifacts`: Defines immutable, versioned, paper-only governor policy artifacts with canonical JSON bytes, canonical hashes, create/get/get-active behavior, duplicate handling, and paper safety gates.

### Modified Capabilities
- None.

## Impact

- Affected code: focused additions under `runtime/strategy` and `runtime/governor`, plus SQLite-backed tests in those packages.
- Runtime dependencies: reuses existing GORM database patterns and shared database configuration; no new external service dependency is expected.
- Public product surfaces: Go runtime package surface only; no HTTP/OpenAPI, backend dependency injection, UI, scheduled job, AI, or live venue changes.
- Storage: adds local runtime database tables for strategy artifacts, governor policy artifacts, and the active paper governor policy selector.
