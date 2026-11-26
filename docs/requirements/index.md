# Requirements

* [Backend process mode requirements](backend-process-modes.md) - API-only startup, split consumers, scheduler ticks, and diagnostic combined startup.
* [Database migration requirements](database-migration-command.md) - Explicit preparation of all configured PostgreSQL schemas.
* [Direct finance CLI requirements](direct-finance-cli.md) - The HTTP-only swmd command surface, credentials, output, and safe retries.
* [Application dispatch requirements](domain-event-pubsub.md) - Generic durable commands, typed events, delivery, and transport ownership.
* [Observed job requirements](durable-jobs.md) - Opt-in product visibility for work executed through appdispatch.
* [Finance fixture requirements](finance-fixtures.md) - Deterministic service-backed finance fixtures for local development and tests.
* [Finance management requirements](finance-management.md) - Tenant-owned ledger, imports, reporting, provider sync, and finance API contracts.
* [Finance UI requirements](finance-operator-ui.md) - Protected tenant-aware Finance workflows, feedback, and supporting admin screens.
* [Personal access token requirements](personal-access-tokens.md) - Self-service token lifecycle, exact route access, and safe API errors.
* [PostgreSQL environment requirements](postgresql-database-environments.md) - The sole supported core database, local roles, bootstrap, and test setup.
* [Finance UI styling requirements](sumweave-ui-bootstrap-rails.md) - Canonical Bootstrap login and Finance styling, layout, and landing behavior.
* [Transaction classification requirements](transaction-classification.md) - Ordered category and additive-tag rules with shared explicit and automatic execution.
* [Transfer matching requirements](transfer-matching.md) - Four deterministic pair rules, complete evidence, uniqueness, and correction.
