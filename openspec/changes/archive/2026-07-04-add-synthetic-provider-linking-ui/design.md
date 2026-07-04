## Context

The finance module already has provider sync v2 link coordination, pending redirect starts, synthetic provider storage, and a synthetic connector. Synthetic can fetch generated account/transaction data, but its only setup path is a core-only helper that directly creates a connection and provider state.

The current API/UI supports Monobank token linking and PKO redirect/SCA linking. Synthetic needs no external auth, but it still needs a link lifecycle: reserve a pending state, let the operator configure synthetic accounts, finish the link, then sync by loading synthetic state later.

## Goals / Non-Goals

**Goals:**

- Reuse the start/finish redirect link lifecycle for synthetic as a local app redirect flow.
- Add tenant/member-protected API endpoints to configure pending synthetic accounts by link `state`.
- Persist synthetic provider state under a stable provider reference key available before the final connection id exists.
- Ensure finished synthetic connections save `ProviderReference` to the same key and fetch using that key.
- Add UI to start synthetic setup, configure accounts, finish the link, and return to the finance connections list.
- Replace the existing manual API e2e workaround with documented API calls.
- Add a dedicated manual UI e2e guide and require run/report/fix iteration during implementation.

**Non-Goals:**

- No external synthetic credentials, OAuth, SCA, or provider network calls.
- No generic provider-specific configuration framework beyond the synthetic workflow.
- No free-text provider entry in UI.
- No backward-compatibility migration work for previous alpha synthetic state keyed by connection id.

## Decisions

1. Reuse redirect start/finish for synthetic as a local redirect flow.

   Synthetic `StartLink` should generate a state key and return the fixed internal browser route `#/finance/connections/synthetic?state=...` as `AuthorizationURL`. The existing pending-start row remains the source of tenant, actor, provider, state, callback URL, expiry, and replay protection.

   Alternative considered: add a separate `configured-link` lifecycle. Rejected for this slice because start/configure/finish already maps cleanly to local setup and avoids another top-level link lifecycle.

2. Key synthetic provider state by `ProviderReference`.

   The state value returned from synthetic `StartLink` becomes the synthetic provider reference. Pending configuration APIs write synthetic state under that key. Synthetic `FinishLink` validates the configured state and returns `LinkResult{ProviderReference: state, Secret: "", State: active}`. The coordinator saves the connection with that provider reference.

   Sync must pass `connection.ProviderReference` through `ProviderSyncParams` into `domain.ProviderConnectionRef.ProviderReference`. Synthetic fetch then loads state by provider reference instead of by bank connection id.

   Alternative considered: store the synthetic state key in connection secret plaintext. Rejected because the value is a provider reference/state key, not credential material.

3. Add a small synthetic-specific configuration API.

   Add protected tenant-scoped endpoints for pending synthetic setup:

   - `GET /api/v1/finance/tenants/{tenantId}/connections/synthetic-link-states/{state}`
   - `PUT /api/v1/finance/tenants/{tenantId}/connections/synthetic-link-states/{state}`

   The `PUT` request accepts configured accounts with `name` and `currency`. The persisted synthetic state should assign stable synthetic account keys so duplicate configured rows with the same `name` and `currency` remain distinct across save/reload/finish/sync round trips. The response returns the state, provider, configured accounts, and whether the setup can be finished. The `GET` endpoint supports refresh/reopen of the setup screen.

   Both endpoints must verify the pending start belongs to the authenticated actor, tenant, and provider `synthetic` before reading or writing config. Expired or consumed states should fail as not found or invalid input without leaking whether another user owns the state.

   Alternative considered: put configured accounts in the redirect finish body. Rejected because it makes refresh/retry awkward and stretches the existing finish contract.

4. Keep finish minimal.

   The existing redirect finish payload can keep `state`; synthetic should ignore or not require `code`. If the generated OpenAPI model currently requires `code`, make it optional for redirect finish and validate provider-specific requirements in service/controller code: PKO requires non-empty code, synthetic requires configured synthetic state.

   Alternative considered: overload `code` with serialized configuration. Rejected because it creates unclear API semantics and makes UI state recovery worse.

5. UI keeps provider-specific panels, not a provider text box.

   `#/finance/connections` should add a synthetic setup action beside Monobank and PKO. The synthetic setup route is `#/finance/connections/synthetic`; it should be tenant-aware, validate at least one configured account, allow adding/removing account rows, persist config through the synthetic configuration API, preserve duplicate configured rows as distinct entries through stable account keys, finish the link, and return to the connection list. Existing connection cards should continue to list and sync synthetic connections the same way as other providers.

   Alternative considered: add a generic provider dropdown. Rejected because current UI requirements explicitly avoid free-text or connector-name entry, and each provider has different setup needs.

## Risks / Trade-offs

- [Redirect terminology is broader than external SCA] -> Treat synthetic as a local redirect link in docs and code comments, and keep PKO-specific SCA wording separate.
- [Pending synthetic config can be mutated by the wrong user] -> Authorize every config read/write against pending start tenant, actor, provider, and state.
- [Synthetic state can orphan if start succeeds but setup is abandoned] -> Let pending starts expire normally; synthetic state keyed by expired unconsumed provider reference can be overwritten on a later state only by using a different generated key, and cleanup can remain out of scope for alpha.
- [Finish can create a connection without configured accounts] -> Synthetic finish must validate the configured state has at least one account with non-empty name and currency.
- [Manual e2e discovers UI/API mismatch] -> Implementation tasks must update guides first enough to run, execute the documented API/UI flows, report findings in the implementation notes or review artifacts, fix issues, and rerun until the guides pass.

## Migration Plan

1. Change synthetic provider state identity from final connection id to provider reference/state key across domain, persistence, linker, and connector fetch.
2. Thread bank connection `ProviderReference` through sync params into connector-backed fetch.
3. Add protected synthetic link-state configuration service/API behavior and generated client types, including stable synthetic account keys for duplicate configured rows.
4. Add synthetic redirect start/finish connector behavior and allow synthetic in redirect provider validation.
5. Add UI synthetic setup route/form at `#/finance/connections/synthetic` and update finance UI behavior docs.
6. Update `docs/manual-e2e/synthetic-provider-flow-e2e.md` to use API start/configure/finish instead of the temporary Go test helper.
7. Create a manual UI guide for synthetic setup and add it to the manual e2e index.
8. During implementation, run the updated API guide and the new UI guide, record findings, address failures, and repeat until both pass.

Rollback before release is removing the new API/UI surface and reverting synthetic state lookup to connection id. Because the project is early alpha, no backward-compatible state migration is required.

## Open Questions

- Should the synthetic config API support deleting a pending state explicitly, or rely on pending-start expiry for this slice?
