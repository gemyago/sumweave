## Context

The finance domain already has provider abstractions for token linking, redirect/SCA start-finish linking, and job-backed sync. `MonobankProvider` implements token linking and sync. `EnableBankingProvider` implements redirect/SCA start-finish and sync. The current app HTTP surface only exposes token linking, and the Finance connections screen lets operators type an arbitrary provider string, so the UI can submit unsupported provider/method combinations such as Enable Banking through a token form.

The product-supported bank choices for this slice are monobank and PKO. PKO is reached through Enable Banking, but operators should not have to know or type connector internals. Local and fake-provider validation should continue to work without requiring a browser-trusted HTTPS certificate.

## Goals / Non-Goals

**Goals:**

- Make supported bank choices explicit: `monobank` and `pko`.
- Keep monobank linking token-based and reject token linking for any other bank choice.
- Add app HTTP routes and UI flow for PKO redirect/SCA through Enable Banking start and finish operations.
- Keep sync execution, secret encryption, raw/provider-original retention, schedule visibility, and sanitized errors on the existing finance/job path.
- Support local/fake-provider Enable Banking flows with `http://localhost` or `http://127.0.0.1` callback URLs.

**Non-Goals:**

- Add additional banks beyond monobank and PKO.
- Build a generic open-banking provider marketplace or arbitrary provider registry UI.
- Require live bank credentials in automated tests.
- Rework finance account/transaction normalization unrelated to the provider-selection gap.

## Decisions

1. Use product-level bank IDs in user-facing flows.

   The UI and app API should expose `monobank` and `pko` as the supported bank choices. Internally, `pko` maps to the Enable Banking connector for start, finish, and sync. This avoids exposing `enable-banking` as a bank provider option while still preserving connector-specific code behind finance-owned provider abstractions.

   Alternative considered: expose `enable-banking` directly and ask the operator to understand that it means PKO. That keeps the current connector naming but preserves the mismatch the change is meant to fix.

2. Make provider/method support explicit at API boundaries.

   `link-token` should only accept `monobank`. Redirect/SCA start and finish routes should only accept `pko`. Unsupported combinations should fail with a bounded validation error before secrets are stored or provider calls are attempted.

   Alternative considered: keep one generic link endpoint with optional token/redirect fields. That would reduce route count but makes invalid states easier to express and harder for the UI to guide.

3. Persist pending redirect link starts server-side.

   PKO/Enable Banking start should create a short-lived pending link record keyed by tenant, actor, bank ID, and state. Finish should load and consume that record before calling the finance service finish path. This prevents trusting provider-reference metadata round-tripped through the browser and makes browser refresh/return behavior deterministic.

   Alternative considered: return all `ProviderLinkStart` details to the UI and require finish to echo them back. That avoids a table but lets client-supplied metadata influence connection creation.

4. Keep local callback validation permissive only for local origins.

   The backend should accept HTTPS callback URLs and local HTTP callback URLs for `localhost` or loopback addresses. This keeps the fake/POC Enable Banking flow usable without trusted SSL while avoiding broad acceptance of arbitrary insecure production callbacks.

   Alternative considered: require HTTPS for every callback. That is simpler but contradicts the working POC/local path and slows validation.

5. Keep supported link options UI-static in this change.

   The Finance connections page should render two explicit product-defined flows: monobank token link and PKO via Enable Banking. This change should not add a separate backend "supported options" discovery endpoint; the backend remains the source of truth by validating route/bank/method combinations and by returning bounded configuration errors if PKO is not available in a given environment.

   Alternative considered: expose supported-link options from the backend and make the page fetch/render them dynamically. That is more flexible, but it adds a new contract and client state surface that is not needed for a two-bank early-alpha slice.

6. Use the existing Finance connections hash route as the PKO SPA return target.

   The UI should send Enable Banking a callback URL shaped as `{origin}/#/finance/connections`, where `{origin}` is the current app origin. Enable Banking then returns to the browser as `{origin}/?code=...&state=...#/finance/connections`. `FinanceConnections.svelte` should treat that hash route plus top-level `code`/`state` query string as the only PKO finish target, call the backend finish route from there, and then clear the consumed query string while keeping `#/finance/connections` active.

   Alternative considered: add a dedicated callback page or a second finance hash route for PKO return handling. That is workable, but it is more UI churn than needed because the existing connections page already owns the linking workflow.

## Risks / Trade-offs

- Pending redirect link records add a small persistence surface. → Keep the model narrow, short-lived, tenant-scoped, and auto-migrated with the finance module.
- PKO is implemented through Enable Banking but represented as `pko` to operators. → Keep the mapping isolated in finance/app wiring and cover it with controller and provider-sync tests.
- Local HTTP callback allowance can be misused if too broad. → Only allow non-HTTPS for loopback/localhost URLs and keep production configuration free to require HTTPS later.
- Live PKO SCA remains hard to automate. → Use fake Enable Banking provider coverage for start/finish/sync and keep manual smoke validation in the UI runbook.

## Migration Plan

- Add/update finance tests first for accepted and rejected provider/method combinations.
- Add pending link persistence and app routes, then regenerate app route code.
- Update the UI client and Finance connections page to use provider-specific flows and the fixed PKO return URL shape.
- Run the existing local DB migration command before starting the stack because finance tables are prepared explicitly.
- Rollback is removing the new routes/UI branch and reverting to the previous token-link-only path; no compatibility migration is needed in early alpha.

## Open Questions

- None for this change.
