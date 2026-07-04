## 1. Provider Reference Storage Foundation

- [x] 1.1 Move synthetic state identity to provider reference/state key, and must follow TDD flow by first adding failing finance tests that prove synthetic state can be saved, loaded, fetched, and updated by provider reference, including round-tripping stable synthetic account keys so duplicate configured accounts remain distinct after persistence and connector-backed fetches, before renaming the domain/store lookup path and threading `ProviderReference` through connector-backed sync.

## 2. Pending Synthetic Configuration Service And Persistence

- [x] 2.1 Add protected synthetic pending-state configuration service and persistence behavior, and must follow TDD flow by first adding failing finance tests proving tenant/actor/provider/state authorization, account validation, pending-state refresh, stable synthetic account key assignment, and duplicate configured accounts with the same `name` and `currency` remain distinct when saved and reloaded by provider reference before implementing the focused service/store methods without expanding the legacy broad store.

## 3. Synthetic Link Lifecycle

- [x] 3.1 Add synthetic local redirect start/finish support, and must follow TDD flow by first adding failing connector and link-coordinator tests proving synthetic start returns the local setup URL/state for `#/finance/connections/synthetic`, finish requires configured synthetic state, finish creates an active connection with `ProviderReference`, PKO redirect finish still requires a non-empty `code` while synthetic may omit it, and unsupported provider/method combinations still fail before secret writes before implementing the connector/coordinator/service changes.

## 4. HTTP And OpenAPI Surface

- [x] 4.1 Expose synthetic configuration and provider-specific redirect-finish validation through the app HTTP API, and must follow TDD flow by first adding failing controller/app registration tests for `GET` and `PUT` synthetic link-state endpoints, PKO requiring non-empty redirect-finish `code` while synthetic may omit it, generated camelCase request/response shapes, auth/tenant isolation, and API round trips that preserve stable synthetic account keys so duplicate configured accounts remain distinct before updating OpenAPI routes, generated models, controller wiring, and app DI.

## 5. Finance UI Flow And Docs

- [x] 5.1 Add generated API client support and UI synthetic setup workflow, and must follow TDD flow by first adding failing UI tests for starting synthetic setup, rendering `#/finance/connections/synthetic` from returned state, loading and saving pending configuration, adding/removing configured accounts, preserving duplicate configured accounts as distinct rows across refresh/save interactions, finishing the link, returning to connections, and displaying the resulting synthetic connection before implementing the Svelte route/form and API wrapper changes.
- [x] 5.2 Update finance UI behavior docs for synthetic setup, and must follow TDD flow by first identifying the expected fixed route and state transitions in `ui-wireframe.md`, then updating the docs alongside the UI implementation and checking the docs remain consistent with the tested UI behavior.

## 6. Manual E2E Documentation And Iteration

- [x] 6.1 Replace the synthetic provider API manual e2e workaround, and must follow TDD flow by first updating `docs/manual-e2e/synthetic-provider-flow-e2e.md` to document start/configure/finish API calls instead of writing a temporary Go test file, then run the updated guide after HTTP/API implementation, report findings in the implementation artifacts, address failures, and rerun until the documented API flow passes.
- [x] 6.2 Add manual UI e2e coverage for synthetic setup, and must follow TDD flow by first creating a dedicated UI guide under `docs/manual-e2e/` and linking it from the manual e2e index, then run the guide after UI implementation, report findings in the implementation artifacts, address failures, and rerun until the documented browser flow passes.
