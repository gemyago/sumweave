---
type: Product reference
title: "Personal finance capabilities"
description: "Current finance workflows, reporting semantics, and product limits."
sources:
  - id: spec
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/openspec/specs/finance-management/spec.md"
    title: "openspec/specs/finance-management/spec.md"
  - id: reporting-go
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/finance/reporting.go"
    title: "finance/reporting.go"
  - id: fx-go
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/finance/fx.go"
    title: "finance/fx.go"
  - id: agents
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/finance/AGENTS.md"
    title: "finance/AGENTS.md"
  - id: documentation-snapshot
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/docs/product/overview.md"
    title: "Reviewed documentation checkpoint before this format migration"
---

# Personal finance capabilities

Sumweave's product direction is personal budgeting and finance tracking. The
current implementation covers finance tracking; budget planning is not yet
implemented.[^documentation-snapshot][^spec][^reporting-go][^fx-go][^agents]

Sumweave tracks personal finances for one or more tenants. A tenant is a shared
financial workspace, such as a household, with a name, display currency,
members, and invitations. All members have equal permissions today. Users can
create, join, update, select, and archive tenants. Catalogs and ledger data are
tenant-owned; current membership is checked for every finance operation.

The protected Finance UI is the default landing area after sign-in. It includes
a dashboard, tenants, accounts, transactions, connections, imports, categories,
tags, rules, and relevant job details. The active tenant is reused across those
routes. Login and Finance use the canonical Bootstrap shell; module-specific
layout and design references remain in the UI module.

## Accounts and ledger

An account belongs to exactly one tenant and has its own currency. Manual,
imported, and linked-bank workflows share the same ledger. Linking can attach a
provider account to an existing manual account. Balances are derived from
transactions; adjustments use visible reconciliation or opening-balance rows.
Provider balance snapshots are observations, not direct balance mutations.[^documentation-snapshot]

Transactions retain source, kind, status, signed minor-unit amount, currency,
effective timestamp, description, optional category, and tags. Members can edit
description, amount, effective timestamp, and category; tags have their own
assignment workflow. Ordinary edits preserve account, currency, source, status,
kind, provider originals, transfer metadata, exclusions, and hidden state.
Provider-original fields and current provider source documents remain separate
from member-edited ledger values. Hiding keeps data for audit and sync identity.

Categories are flat and tenant-local; tags add cross-category detail. Defaults
are copied when a tenant is seeded and later default changes do not rewrite
existing catalogs. Live classification-rule references block category removal
or tag hiding until those rules are retargeted or deleted.

[Classification](classification.md) assigns a category and optional missing tags
through ordered deterministic rules. [Transfer matching](transfer-matching.md)
links qualifying pairs, and manual link/unlink remains available for correction.
Neither creates payments or moves money.

## Bank connections and current provider data

- Monobank uses personal-token linking.
- PKO uses Enable Banking redirect/SCA linking.
- Synthetic is a configured local redirect provider for deterministic development
  and verification, not a real bank connection.

Connections track provider and connector identities, encrypted credentials,
re-authentication/expiry state where available, sync diagnostics, and schedules.
On-demand and scheduled sync share the same
[requested-window pipeline](../finance-provider-sync-architecture.md).
A newly active connection receives an enabled daily schedule with its first run
24 hours after linking. Scheduler ticks repair missing schedules without changing
existing state. Linking alone does not execute the scheduled occurrence.[^documentation-snapshot]

Current provider source data is a sanitized, schema-derived snapshot for the
connection, account, balance, or transaction. Distinct document kinds have
separate identities. A newer snapshot replaces the same identity; this is not
raw HTTP response retention or a historical document timeline. List endpoints
return metadata; detail endpoints return a sanitized document after tenant and
attachment checks. Secrets are encrypted at rest and excluded from snapshots,
logs, and user-facing job data.

## CSV migration

Account import and transaction import have preview/validation, explicit
confirmation, durable execution, and finance-owned audit/result views. Accounts
are resolved from tenant-local names so a transaction file can span accounts.
Repeat imports use the existing deduplication behavior.[^documentation-snapshot]

Transaction CSV input has seven fixed required headers:
`Date`, `Account`, `Category`, `Tags`, `Expense amount`, `Income amount`, and
`Currency`. `Description` is optional. The raw file limit is **250,000 data rows**
and **64 MiB**. There is no general column-mapping or combined-import promise in
the current contract. See the detailed [transaction import guide](../manual-e2e/finance-transaction-csv-import-e2e.md)
and [account import guide](../manual-e2e/finance-account-csv-import-e2e.md) for
exact date/amount forms, preview fields, confirmation, rejected rows, and reruns.
Manual entry, import, and rule edits do not themselves trigger enrichment;
explicit runs or a committed bank-sync window do.

## Reporting and FX

The dashboard provides period-based settled income, expenses, net flow,
category breakdowns, pending context, account balances, native-currency totals,
and missing-FX diagnostics. Current persisted FX rates convert into the tenant's
display currency. Historical transaction-date FX reconstruction is not promised;
the initial design's historical-rate target is superseded by current-rate behavior.
Frankfurter is the implemented configured provider; NBP/ECB constructors remain
stubs that return `ErrFXProviderNotImplemented`, not working fallback integrations. FX refresh is explicit or driven by finance-owned daily
schedule state. Missing conversions remain visible as incomplete totals.[^documentation-snapshot]

- Booked visible ordinary transactions contribute by amount sign.
- Refunds reduce expense in their category rather than becoming ordinary income.
- Matched internal transfers affect account balances but are excluded from
  income/expense; unmatched transfers contribute by sign.
- Reconciliations and opening balances affect balances, not income/expense.
- Pending items are visible separately from settled totals; hidden items are excluded.

The focused `GET /api/v1/finance/tenants/{tenantId}/cash-flow-series` resource is
separate from the dashboard. It accepts full `startDate`/`endDate` timestamps
(inclusive/exclusive) and `groupBy=day|month`, with at most **366 buckets**.
PostgreSQL emits ordered, zero-filled buckets anchored to the supplied start.
Monthly boundaries derive from the original start day/time and fixed supplied
offset, not a previously clipped month or the database session calendar.
There is no separate timezone parameter; DST calendar handling is deferred.
Only booked rows visible through visible accounts contribute. It uses current
configured-provider FX, rounds each converted row with `math.Round` before
summing, omits unavailable conversions, and marks the response incomplete with
currency-pair diagnostics. The UI chart uses the same focused resource and native
ECharts legend controls. This reporting date contract differs from enrichment's
local-calendar date-to-offset conversion.

## Access, background feedback, and limits

Browser sessions serve the full protected app. Self-service personal access
tokens and the HTTP-only `swmd` client expose a deliberately narrow existing-route
allowlist; see [direct access](../integrations/direct-access.md). Admin access-token
management and finance diagnostics are supporting app functions. Generic Chat,
agent profiles/execution, and model-provider configuration are
[experimental](../experiments/agent-runtime.md).[^documentation-snapshot]

Asynchronous triggers return a dispatch message ID, materialized as a job on
first worker delivery. Only a flow holding an expected new ID treats its early
`404` as pending. Jobs show lifecycle and sanitized errors, not generic results
or provider payloads. See [background processing](../architecture/background-processing.md).
Shared provider clients retry safe `429` responses once, using `Retry-After` or
fallback delay. Concurrent classification edits and transfer-matching races are
accepted Phase 0 limitations documented on their respective pages.

The current early-alpha delivery covers tracking rather than budget planning.
Trading, forecasting, automated advice, LLM classification, and richer tenant
roles are not implemented capabilities. No budget-planning workflow or delivery
schedule is specified here. Breaking API and migration changes are accepted
during alpha.

## Contracts and verification

Requirements below are in this bundle; repository implementation and UI references are external:[^documentation-snapshot]

- [Finance requirements](../requirements/finance-management.md)
- [Operator UI requirements](../requirements/finance-operator-ui.md)
- [Fixture requirements](../requirements/finance-fixtures.md)
- [Canonical login and Finance shell](../requirements/sumweave-ui-bootstrap-rails.md)
- [HTTP/OpenAPI contract](../../apps/sumweave/internal/api/http/v1routes.yaml)
- [Ledger/reporting implementation](../../finance/reporting.go)
- [UI design system](../../apps/sumweave-ui/DESIGN.md) and [screen reference](../../apps/sumweave-ui/ui-wireframe.md)

Start verification from the [manual guide index](../manual-e2e/index.md): tenancy,
balances, entry, linking, import, FX, scheduling, and shell journeys each have
focused runbooks. [Development verification](../development/verification.md)
links the automated finance smoke suite. These are procedures, not a record of
new E2E results.

[^documentation-snapshot]: Reviewed documentation checkpoint, preserved as the source of this section; format and navigation changes do not establish execution results.

[^spec]: openspec/specs/finance-management/spec.md.
[^reporting-go]: finance/reporting.go.
[^fx-go]: finance/fx.go.
[^agents]: finance/AGENTS.md.
