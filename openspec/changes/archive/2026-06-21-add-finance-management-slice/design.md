## Context

Signal Foundry's accepted repo direction remains `runtime/` as the deterministic trading core, `apps/signal-foundry/` as the single Go API/jobs application, and `apps/signal-ui/` as the single SPA. Issue #29 and `docs/finances-management/design.md` define a new finance slice that must live beside the trading runtime rather than inside it. The same issue also makes generic durable jobs a prerequisite, because finance needs visible on-demand and scheduled bank sync, FX sync, and CSV import execution from day one.

The repo already contains two useful foundations:

- finance provider POC knowledge for Enable Banking / PKO and monobank
- an app-owned durable historical backfill jobs foundation

Neither is sufficient as-is for the requested product slice. This change therefore plans one unified implementation that first generalizes the app jobs substrate, then builds the finance module, APIs, UI, fixtures, and end-to-end validation around it.

## Goals / Non-Goals

**Goals:**

- Create a production-quality `finance/` root module that is independent from `runtime/`.
- Promote the current historical-backfill jobs runtime into a generic app-level durable jobs substrate with worker and scheduler modes.
- Implement tenant-based finance management with tenant create/invite/join/member flows, ledger-driven accounts and transactions, categories, tags, reporting, FX conversion, bank sync, CSV imports, and fixture generation.
- Productize Enable Banking / PKO and monobank behind finance-owned connector abstractions.
- Expose finance APIs through the existing backend app and finance/admin workflows through the existing UI.
- Provide dedicated automated and manual end-to-end validation, including a fix-iterate loop until the finance slice is operational across the full stack.

**Non-Goals:**

- No finance code inside `runtime/`.
- No compatibility layers, dual writes, or legacy migration paths for pre-finance schemas.
- No payment initiation, budgeting, forecasting, automated advice, tenant roles, or per-tenant encryption keys.
- No separate finance backend process; the app remains the single API/jobs process owner even when worker and scheduler run as separate modes of the same binary.
- No replacement of trading runtime architecture or mixing finance flows into trading/operator workflows.

## Decisions

1. Keep finance as a root product module, not a runtime slice.

   Create `finance/` as a root Go module that owns finance domain types, application services, persistence models, migrations, connector interfaces, provider implementations, CSV import logic, reporting queries, and fixture generation. `finance/` must not import `runtime/`. `apps/signal-foundry/` composes both modules and owns auth, config, process lifecycle, jobs runtime, and HTTP route glue.

2. Generalize app durable jobs before adding finance jobs.

   Refactor the current historical-backfill-specific jobs implementation in `apps/signal-foundry/` into a generic app-level substrate with:

   - persisted dot-namespaced job types such as `data.historical_raw_candle_backfill`, `finance.bank_connection_sync`, `finance.fx_rates_sync`, `finance.csv_import`, and `finance.account_import`
   - generic `input_json`, `result_json`, and optional `progress_json`
   - typed handler registration in the app layer
   - persisted requester/source/correlation/idempotency metadata
   - statuses `queued`, `running`, `succeeded`, `failed`, and `canceled`
   - transactional claiming, stale-running recovery, attempts/max-attempts, and sanitized errors
   - a worker process mode such as `signal-foundry jobs worker`
   - a scheduler tick mode such as `signal-foundry jobs enqueue-due`
   - a database-backed schedule registry used by finance recurring sync and future product schedules

   Persisted `job_type` values and app handler registration must use that single dot-namespaced scheme consistently even when external route names or UI copy differ. The API process enqueues and serves status; durable work does not execute inline in normal API requests. Per-connection bank sync schedules are finance-domain records managed from finance connection workflows, while FX schedules are system/admin-managed registry entries surfaced in admin diagnostics; both scheduled and manual executions produce normal visible job records.

3. Preserve historical backfill behavior through the generic jobs path.

   The historical raw candle backfill runner remains owned by `runtime/flows`, but the app registers it as a typed generic job handler. The Data page remains explicit and read-only by default; backfill is still a deliberate mutation that creates a durable job rather than ingesting inline.

4. Model finance storage as finance-owned tables with separate domain and persistence models.

   Finance data uses GORM auto-migrate for finance-owned tables, UTC-first timestamps, `finance_` table prefixes, separate GORM models from domain models, SQLite local compatibility, and PostgreSQL-friendly schemas. Raw provider/import payloads are stored because the design requires them for audit/debugging, but they remain sensitive data.

5. Treat ledger semantics as the core finance invariant.

   Accounts are ledger-driven from transactions. Users do not directly edit balances. Corrections happen through visible reconciliation or opening-balance transactions. Provider balance snapshots are observations; if the ledger must align to a provider snapshot, the system creates explainable reconciliation transactions instead of mutating balances out of band.

6. Preserve provider truth separately from user-edited finance presentation.

   Synced transactions keep provider-original values and raw payloads separate from user-edited fields. Sync idempotency uses provider, connection, provider account, provider transaction id, and stable fallback fingerprints for pending-to-booked transitions. Provider-synced delete behavior means hide/exclude rather than hard delete.

7. Make FX-backed reporting reproducible.

   The tenant display currency drives finance dashboards and summaries. Persist FX rates used for reporting instead of relying on live lookups. Use Frankfurter as the default provider, with narrow provider interfaces that also allow NBP and ECB sources later. Missing rates are surfaced clearly in APIs and UI rather than silently producing misleading totals.

8. Treat CSV import as an explicit preview-confirm-job workflow.

   CSV import is not a blind upload endpoint. The required flow is: upload -> detect headers -> propose mapping -> validate/preview -> show duplicates and would-create effects -> explicit confirmation -> durable import job -> progress/result/rejected-row visibility. Account-only import must work independently from transaction import.

9. Reuse the existing backend and UI surfaces without mixing product boundaries.

   Backend routes live under `/api/v1/finance/...` plus generic `/api/v1/jobs...` routes. Required finance endpoints include tenants, tenant members/invites, accounts, connections, transactions, categories, tags, dashboard/reporting, FX diagnostics/sync, and import preview/confirm/status flows. The backend app remains thin and delegates business behavior into `finance/`. The UI adds a clearly distinct Finance area with tenant-aware routes for tenant selection/create/invite/join/member visibility, dashboard/accounts/transactions/imports, and bank-linking flows including Enable Banking redirect/SCA, monobank token entry, attach-to-existing-account, re-authentication, per-connection schedule management, and job history. Admin routes stay utilitarian and sanitized for generic jobs plus FX/provider diagnostics; they provide cross-cutting visibility and global FX controls rather than replacing tenant finance workflows.

10. Add only minimal fixture scaffolding early, then complete realistic fixture generation after core services exist.

    Early chunking should add only the seed/config/scenario-builder seams needed for module and integration work without depending on unfinished finance services. After tenant/account/transaction/reporting/provider/import services exist, implement `signal-foundry finance fixtures generate ...` in the app binary with scenario generation logic inside `finance/`. The completed CLI must call finance services rather than writing tables directly. Generated scenarios must then cover multi-currency tenants, member/invite states, accounts, linked/manual connections, pending/booked transactions, refunds, transfers, reconciliations, hidden/deleted items, FX data, and representative job states so UI and e2e flows have realistic data.

11. Make security and privacy explicit in storage, APIs, logs, and UI.

    Provider credentials and session secrets are encrypted at rest with one system symmetric key for the first implementation. Secrets must never appear in logs, API responses, job status payloads, or default admin UI surfaces. Provider errors and job errors must be sanitized, and raw payloads must be treated as sensitive personal data.

12. Update architecture and operations docs as part of delivery.

    The implementation must update `docs/ARCHITECTURE.md` where the product shape changes, local/backend run instructions for API plus worker/scheduler modes, production/ops notes for scheduler ticks, fixture CLI usage, manual e2e guidance, and any AGENTS instructions affected by changed commands or workflows.

13. Build validation around one dedicated e2e phase plus a fix-iterate loop.

    The implementation plan must end with a finance-specific end-to-end phase that exercises fixture generation, API + worker + scheduler flows, dashboard/accounts/transactions/imports/connections/jobs UI behavior, and generic admin diagnostics. That phase should first land failing automated coverage, then manual/browser smoke runbook updates, then repeated fix-and-rerun work until the full flow is stable. This is not optional because the change spans backend, worker modes, provider seams, OpenAPI/UI integration, and user-facing workflows.

## Risks / Trade-offs

- The generic jobs refactor is the main dependency risk because it changes existing app job assumptions and introduces worker/scheduler modes.
- Finance scope is broad. Clear chunk ordering, early minimal scaffolding, and later realistic fixture completion are necessary to keep UI and e2e work grounded in real workflows without violating dependencies.
- Live bank verification is constrained: Enable Banking / PKO requires human SCA, and monobank has tight rate/range limits. Offline fake-provider coverage therefore carries most validation weight.
- CSV import can sprawl into dialect heuristics; the first implementation should optimize for explicit mapping and deterministic preview/confirm behavior instead.
- Finance dashboards can become visually dense; the UI must follow the repo rule to prefer summary cards and focused detail routes over split panes or oversized tables.

## Migration Plan

1. Refactor app jobs into the generic substrate and migrate historical backfill to typed handler registration.
2. Add the `finance/` module skeleton, auto-migrate schema initialization, encryption primitives, and minimal fixture scaffolding.
3. Implement finance core services, reporting, FX, connectors, schedules, sync/import jobs, and tenant/member/invite APIs in the documented chunk order.
4. Complete realistic fixture generation, finance UI flows, and worker/scheduler/fixture operational docs once the dependent services exist.
5. Finish with dedicated finance e2e coverage and a fix-iterate pass until the end-to-end flow is stable.

Early alpha policy applies throughout: destructive migration reshaping and API changes are acceptable if they move the repo to the cleaner target architecture.

## Open Questions

- No planning blockers are currently identified.
- Exact route names, table splits for provider-original/raw-payload records, and handler-specific cancel/retry semantics can be finalized during implementation as long as they stay within the design constraints above.
