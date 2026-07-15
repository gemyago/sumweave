## Finance module

Root Go module for the finance product slice.

## Boundary

- `finance/` is product scope and must stay independent from `runtime/`.
- Keep finance domain models separate from persistence/GORM models.
- Use GORM auto-migrate for finance-owned tables; avoid custom SQL migrations.
- Keep secrets encrypted at rest and never persist plaintext credentials.

## Architecture

### Config

- `finance/finance_cfg.go` is a top-level module config definition. It includes all the required configs and validation methods.

### Wireup

- `finance/finance.go` is a top-level module wireup entry point. It should take required config and/or external dependencies and wire-up/expose all the required services.
- outside world should only use services exposed by `Finance` instance (from `finance/finance.go`)
- The `Finance` should expose minimal services required for outside world to work.

### Public Services

- Public services must expose minimal shape required for outside world to operate.
- Each service is single-responsibility

## Rules

- Do not extend persistence/store.go with new methods, this is a legacy over-engineered "god object" that should be phased out. Instead create new dedicated stores for each responsibility.
- Transaction CSV imports cap raw CSVs at 250,000 data rows and 64 MiB.

## Commands

- `make test`
- `make lint`

To fix lint errors, attempt: `golangci-lint run --fix` and then manually fix the remaining errors (if still present).

## Task Completion Protocol

Repository-level completion protocol must be followed. Always report task completion status as per repository-level protocol.
