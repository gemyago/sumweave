Chunk ordering: complete sections 1 through 2 strictly in order; do not start a later chunk until focused tests for the current chunk have been written, made to fail, implemented, and passed.

## 1. Finance Tenant Archive Domain And API (`finance-tenant-archive-domain-and-api`)

- [x] 1.1 Add finance tenant soft-archive lifecycle behavior and must follow TDD flow by first writing failing finance-domain and persistence tests proving a current tenant member can archive a tenant, the archive state is persisted without deleting tenant-owned data, and archived tenants no longer appear in `ListTenantsForUser` before implementing and verifying focused tests.
- [x] 1.2 Add the protected finance tenant archive HTTP surface and must follow TDD flow by first writing failing registered-route controller/API tests proving authenticated members can call the new archive endpoint, the archive mutation stays on the existing camelCase `/api/v1/finance/...` surface without returning tenant data, and subsequent tenant list/get calls no longer expose the archived tenant before implementing and verifying focused tests.

## 2. Manual E2E Coverage (`finance-tenant-archive-manual-e2e`)

- [x] 2.1 Add manual API e2e documentation for tenant management create/list/archive and must follow TDD flow by first extending the failing backend/API tests needed to lock the documented create, list, archive, and post-archive list behavior, then updating `docs/manual-e2e/README.md` and adding a tenant management guide that documents the human-run API-only verification flow with no UI steps before closing the change.
