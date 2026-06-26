## Finance module

Root Go module for the finance product slice.

## Boundary

- `finance/` is product scope and must stay independent from `runtime/`.
- Keep finance domain models separate from persistence/GORM models.
- Use GORM auto-migrate for finance-owned tables; avoid custom SQL migrations.
- Keep secrets encrypted at rest and never persist plaintext credentials.

## Commands

- `make test`
- `make lint`

## Task Completion Protocol

Repository-level completion protocol must be followed. Always report task completion status as per repository-level protocol.
