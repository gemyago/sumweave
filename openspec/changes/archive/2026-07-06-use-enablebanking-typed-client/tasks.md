## 1. Typed Client Contract

- [x] 1.1 Add a typed Enable Banking connector client seam, and must follow TDD flow by first writing failing connector tests or compile-focused tests proving the connector dependency exposes typed auth, session, balance, and transaction operations instead of `DoRawObject`, then replacing the raw `apiClient` interface with a narrow typed interface backed by the generated client.
- [x] 1.2 Remove connector dependence on raw-only response fields, and must follow TDD flow by first writing failing connector tests proving values absent from existing typed client models are not recovered through raw maps, then updating connector mapping to use only existing generated typed model fields.

## 2. Connector Refactor

- [x] 2.1 Refactor redirect start and finish to use typed client methods, and must follow TDD flow by first writing failing connector tests proving `CreateAuth` and `CreateSession` are called and raw response maps are not read before implementing typed start/finish mapping.
- [x] 2.2 Refactor fetch to use typed session, balance, and transaction methods, and must follow TDD flow by first writing failing connector tests proving `GetSession`, typed account handling, `GetAccountBalances`, and paged `GetAccountTransactions` produce provider account, balance, transaction, provider-original, fingerprint, and continuation behavior without raw response access before implementing typed fetch mapping.

## 3. Cleanup And Verification

- [x] 3.1 Remove obsolete connector-local raw request helpers, and must follow TDD flow by first adding or updating focused tests that fail while normal connector behavior can still call raw transport helpers for start, finish, or fetch, then deleting unused raw path/query/request helpers from the connector while preserving finance-domain normalization helpers.
