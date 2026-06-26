## 1. Finance OpenAPI Contract Alignment

- [x] 1.1 Add the missing finance query-parameter contract in `apps/signal-foundry/internal/api/http/v1routes.yaml` and regenerate the finance apigen surface, and must follow TDD flow by first writing failing controller tests proving `includeHidden`, transaction list filters, and dashboard period query inputs are accepted through generated finance routes before updating the OpenAPI spec, running `go generate ./internal/api/http/register.go`, and verifying focused tests.

## 2. Finance Controller Builder Refactor

- [x] 2.1 Refactor the tenant, account, category, tag, and transaction finance handlers in `apps/signal-foundry/internal/api/http/v1controllers/finance.go` to use generated `builder.HandleWith` implementations directly, and must follow TDD flow by first extending finance controller tests proving authenticated routes keep tenant scoping, camelCase payloads, and current service parameter mapping after manual body, path, and query parsing are removed before implementing and verifying focused tests.
- [x] 2.2 Refactor the finance connection, dashboard, FX, and CSV import handlers to use the apigen pipeline and remove redundant finance-only wrapper helpers, and must follow TDD flow by first extending finance controller tests proving dashboard date validation, FX diagnostics and sync, bank connection sync, CSV import conflict translation, and missing-caller unauthorized behavior still work through builder-backed handlers before implementing, deleting obsolete wrapper/serialization helpers, and verifying focused tests.
