## 1. Shared Domain Records

- [x] 1.1 Add governor decision domain records following TDD flow: write `runtime/domain` tests for decision status, reason, UTC decision time, candidate action retention, and invalid values before implementing the canonical types and constructors.
- [x] 1.2 Add execution command/order/fill/reconciliation domain records following TDD flow: write `runtime/domain` tests for identities, statuses, quantities, prices, UTC event times, approved-decision linkage, and invalid values before implementing the canonical types and constructors.
- [x] 1.3 Preserve existing domain behavior following TDD flow: extend existing domain tests around strategy candidate actions only where needed to prove the new records do not change current analytics or strategy contracts, then make the minimal implementation adjustment if a regression is exposed.

## 2. Governor Layer

- [x] 2.1 Add governor service request and policy validation following TDD flow: write `runtime/governor` tests for unsupported minimum quality, empty allowed action set, and negative maximum approved action count before implementing validation and `ErrValidation` behavior.
- [x] 2.2 Implement deterministic governor evaluation following TDD flow: write tests for stable repeated evaluation and decision ordering by candidate decision time before implementing sorting and decision creation.
- [x] 2.3 Implement initial policy rules following TDD flow: write tests for approved eligible actions, rejected disallowed action kinds, rejected below-minimum quality, and blocked actions after the approval limit before implementing rule evaluation.
- [x] 2.4 Enforce governor boundaries following TDD flow: write package-level tests that construct the service from canonical candidate actions without venue, AI, execution, persistence, backend, or UI dependencies before keeping the implementation dependency-free.

## 3. Execution Layer

- [x] 3.1 Add execution command creation following TDD flow: write `runtime/execution` tests for approved decisions creating commands, rejected decisions refused, blocked decisions refused, missing malformed decisions refused, positive quantity validation, and stable command identity before implementing the service method.
- [x] 3.2 Add local order recording following TDD flow: write tests for command-linked orders with venue, client order identifier, positive quantity, UTC event time, and invalid input rejection before implementing order record creation.
- [x] 3.3 Add local fill recording following TDD flow: write tests for order-linked fills with fill identifier, positive quantity, positive price, UTC event time, and invalid input rejection before implementing fill record creation.
- [x] 3.4 Add deterministic reconciliation following TDD flow: write tests for open, partially-filled, filled, and overfilled states plus fill ordering by event time and fill identity before implementing reconciliation.
- [x] 3.5 Enforce execution boundaries following TDD flow: write tests that exercise command, order, fill, and reconciliation behavior without live venue credentials, signing, network calls, persistence, backend, UI, or AI dependencies before keeping the implementation local-only.

## 4. Documentation And Scope Checks

- [x] 4.1 Update runtime public-contract documentation if exported API surface changes following TDD flow: first add or adjust the tests that require the exported surface, then update `runtime/AGENTS.md` only for newly exported product APIs that should be documented.
- [x] 4.2 Confirm no out-of-scope surfaces were added following TDD flow: add regression tests or compile checks as needed to prove no new backend routes, migrations, UI code, live venue trading calls, or orchestration runner are required by the new slices.
