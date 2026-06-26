## Context

The current finance slice already supports tenant-scoped transaction listing and manual transaction creation, and the finance service already exposes a narrow transaction update operation that preserves `providerOriginal` values. The remaining gap is that the backend app does not expose transaction detail/update HTTP paths, the generated client has no dedicated detail/update helpers, and the operator UI keeps transactions on a single list/create route with no focused create/edit flow.

This change crosses `finance/`, `apps/signal-foundry/`, and `apps/signal-ui/`. It also needs to stay aligned with the finance product direction in [docs/ARCHITECTURE.md](/Users/jenya/projects/signal-foundry/docs/ARCHITECTURE.md), the existing finance-management and finance-operator-ui specs, and the UI module rule to prefer separate detail routes over dense split-pane workspaces.

Two implementation constraints shape the design:

- Finance edits must remain tenant-scoped and preserve provider-origin truth instead of overwriting it.
- The existing domain update contract only edits `description` and `amountMinor`, while the requested UI workflow also needs `effectiveAt` and `categoryId` editing.

## Goals / Non-Goals

**Goals:**

- Add a focused finance transaction edit route so an operator can open an existing transaction, review its current state, edit user-controlled fields, and save or cancel without leaving the finance workspace.
- Add a dedicated finance transaction create route so an operator can record a new transaction on a screen designed for single-record entry rather than inside the list view.
- Reuse one transaction editor screen for both create and edit modes so mobile and desktop behavior stay consistent.
- Expose a tenant-aware backend API for transaction updates and extend the finance service update contract to cover the editable reporting fields used by the UI.
- Surface provider-original values for synced transactions so the edit screen makes it clear what came from the provider versus what the operator changed.
- Keep pending, hidden, transfer, refund, and reconciliation cues visible while creating or editing so reporting impact stays legible.

**Non-Goals:**

- Do not introduce bulk transaction editing, inline list-row editing, or a split-pane workspace.
- Do not keep the creation form embedded on the transactions list page.
- Do not make account, source, currency, kind, transfer linkage, hidden state, or provider identifiers editable in this change.
- Do not add a new sync conflict-resolution workflow or a provider payload inspection surface.
- Do not redesign the broader finance navigation or reporting flows outside the transaction list/detail workflow.

## Decisions

### 1. Use dedicated create and edit routes backed by one transaction editor screen

The UI will keep `#/finance/transactions` as a browse/filter route and add dedicated editor routes at `#/finance/transactions/new` and `#/finance/transactions/:transactionId`. Both editor routes will reuse one transaction editor screen component with create and edit modes.

Why:

- The UI module guidance explicitly prefers separate detail routes.
- Direct routes support refresh and deep-link behavior better than transient modals or expanded cards.
- A shared editor screen keeps field layout, validation, and mobile behavior consistent between create and edit.
- The list page stays focused on discovery and state scanning instead of competing with a large form.

Alternatives considered:

- Keeping the create form inline on the transaction list was rejected because it overloads the browse route and is especially awkward on smaller screens.
- Separate create and edit screens with duplicated form implementations were rejected because they would drift in behavior and responsive layout over time.
- A modal editor was rejected because it creates weaker deep-link behavior and poorer recovery on reload/direct entry.

### 2. Add one tenant-scoped transaction update endpoint with a full editable-field payload

The backend app will expose `PATCH /api/v1/finance/tenants/{tenantId}/transactions/{transactionId}`. The request body will represent the complete editable reporting projection for the screen: `description`, `amountMinor`, `effectiveAt`, and nullable `categoryId`.

Why:

- The UI edits a coherent set of mutable reporting fields together, so one request keeps save semantics straightforward.
- `PATCH` fits the intent of changing only the mutable subset rather than replacing the whole transaction record.
- Nullable `categoryId` makes category removal explicit without exposing unrelated transaction fields.

Alternatives considered:

- `PUT` with the full transaction shape was rejected because it would imply mutability for fields we want to keep read-only.
- A generic partial patch document was rejected because it adds controller and validation complexity without user value in this scope.

### 3. Expand the finance service update contract, but keep immutable ledger identity locked down

`finance.UpdateTransactionParams` and the ledger service update flow will be expanded to validate and persist `effectiveAt` and `categoryId` alongside `description` and `amountMinor`. Category changes must remain tenant-local and optional so operators can also clear a category.

The update flow will continue to preserve:

- `providerOriginal`
- account identity
- source
- kind
- currency
- transfer linkage
- hidden state
- service-managed timestamps

Why:

- The proposal explicitly calls for editing descriptions, amounts, categories, and effective dates.
- Keeping the mutable subset narrow preserves explainable ledger behavior and avoids turning this route into a general-purpose transaction rewrite tool.

Alternatives considered:

- Leaving domain updates limited to description/amount and faking the rest in the UI was rejected because it would create a misleading edit surface.
- Allowing edits to source/kind/currency/account was rejected because those changes have wider ledger and reconciliation implications.

### 4. Add a dedicated transaction detail read endpoint for edit-route hydration

This change will add `GET /api/v1/finance/tenants/{tenantId}/transactions/{transactionId}` for the shared transaction editor. The edit route will load the selected transaction through that tenant-scoped detail endpoint plus the existing account/category catalog reads. The create route will use the existing account/category catalog reads and initialize a blank editor state.

Why:

- Direct-entry edit routes need a stable way to load exactly one transaction without depending on the current list contents.
- The create screen only needs existing catalog/account context, so it does not need additional backend reads.
- A dedicated detail endpoint keeps list and editor responsibilities separate and scales better once filtering, paging, or larger transaction histories arrive.
- The edit screen can use the same canonical response shape whether the user arrived from the list or loaded the route directly.

Alternatives considered:

- Reusing the transaction list read model to find one record was rejected because direct-entry edit routes would pay for unrelated list data and become fragile once list filters, paging, or larger histories appear.

### 5. Include provider-original context in transaction responses used by the edit flow

The finance transaction JSON returned by list/update flows will expose an optional camelCase `providerOriginal` block so the UI can show original synced description, amount, currency, and effective time when present.

Why:

- The edit screen must distinguish synced provider truth from operator-edited reporting fields.
- Reusing the same response shape across list and update flows keeps client mapping and tests simpler.

Alternatives considered:

- Computing a separate UI-only comparison model was rejected because it would duplicate backend truth and make direct-entry edit routes harder to hydrate.

### 6. Make the transaction editor mobile-first instead of list-first

The shared create/edit screen will use a single-column, touch-friendly editor layout first, then expand progressively for larger screens. Primary actions, field grouping, and provider-context display will be designed so the screen works well without relying on wide multi-column layouts.

Why:

- The user explicitly wants the dedicated creation flow to be mobile friendly.
- Single-record editor screens adapt more naturally to mobile than mixed list-plus-form pages.
- Mobile-first constraints also improve clarity on desktop by forcing cleaner grouping and action hierarchy.

Alternatives considered:

- Reusing the existing list-page layout with responsive tweaks was rejected because the core information architecture is still overloaded.
- Building a desktop-first two-column editor was rejected because it would likely compress poorly on phones and smaller tablets.

## Risks / Trade-offs

- **[Extra API surface]** Adding a dedicated detail endpoint increases controller/OpenAPI/client surface area. → Mitigation: keep the response shape aligned with existing transaction JSON so the endpoint stays small and easy to test.
- **[Route proliferation]** Adding `new` and `:transactionId` routes increases routing surface in the finance area. → Mitigation: keep both routes backed by one editor screen and one shared tenant-resolution path.
- **[Mutable-field drift]** Future finance edits could add more user-editable fields and leave API/UI/service payloads out of sync. → Mitigation: define one explicit editable projection in OpenAPI, client types, and controller/service tests.
- **[Reporting-impact edits]** Changing amount, category, or effective date can materially affect summaries. → Mitigation: keep save/cancel explicit, retain transaction state badges on the edit page, and expose provider-original values when present.
- **[Null category semantics]** Clearing a category can be ambiguous if transport rules are loose. → Mitigation: use explicit nullable `categoryId` handling and cover both assign and clear paths in API/service tests.
- **[Responsive regression]** A create/edit screen can still become awkward on small screens if state badges and comparison data crowd the form. → Mitigation: require mobile-friendly layout coverage in UI tests and wireframe updates, with provider context presented as stacked secondary content.

## Migration Plan

No persistence migration is required for this change because the existing transaction model already stores `effectiveAt`, `categoryId`, and `providerOriginal`.

Implementation sequence:

1. Extend finance service update params/validation and service tests.
2. Add OpenAPI route/schema updates plus controller wiring and controller tests in `apps/signal-foundry/` for transaction detail and update paths.
3. Regenerate the UI API client surface if required by the existing module workflow.
4. Add the shared transaction editor routes for create and edit, detail/update API helpers, mobile-friendly form behavior, and UI tests.
5. Remove inline creation from the transactions list route and replace it with clear create/open actions.
6. Update the finance wireframe/documentation to reflect the dedicated create/edit transaction routes.

Rollback:

- Remove the new HTTP route and the dedicated transaction editor routes together if implementation needs to be backed out.
- No stored-data rollback is required because the change only broadens how existing fields are updated.

## Open Questions

None that block implementation.
