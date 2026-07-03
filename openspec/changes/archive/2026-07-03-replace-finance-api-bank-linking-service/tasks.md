Chunk ordering: complete sections 1 through 3 strictly in order; do not start a later section until focused tests for the current section have been written, made to fail, implemented, and passed.

## 1. Focused Bank-Connection Service

- [x] 1.1 Add a public `finance.BankConnectionService` for bank-link workflows, and must follow TDD flow by first writing failing focused finance tests proving tenant access is enforced, unsupported provider/linking-method combinations fail before any encrypted secret write or connector call, and Monobank token link plus PKO redirect start/finish delegate through provider sync v2 link coordination while preserving the existing public parameter/result semantics before implementing the service and narrow constructor dependencies.
- [x] 1.2 Compose real v2 bank-link dependencies inside package `finance`, and must follow TDD flow by first writing failing tests proving real Monobank and Enable Banking connector registries, product provider profiles, dedicated link persistence, encrypted secret writing, ID/time providers, and pending-start lookup can be constructed without app code importing finance internal packages before implementing the composition helpers.

## 2. API And App Wiring Cutover

- [x] 2.1 Route finance bank-link HTTP handlers to the focused bank-connection service, and must follow TDD flow by first updating controller tests to prove the existing token-link and redirect start/finish routes keep the same request and response shapes while invoking the new controller dependency instead of root `finance.Service` before implementing the controller dependency split.
- [x] 2.2 Wire app DI and callback lookup through the focused bank-connection service, and must follow TDD flow by first updating app registration tests to prove configured Monobank and Enable Banking provider settings build the v2-backed bank-connection service and the Enable Banking callback bridge resolves pending starts through it before implementing the app composition changes.

## 3. Legacy Link Path Removal

- [x] 3.1 Remove the old root-service bank-link path from the protected API handlers and callback bridge, and must follow TDD flow by first updating or deleting service/app tests that assert root `finance.Service` owns token-link, redirect-link, or pending-start callback lookup behavior, then removing the now-unused root-service bank-link methods and legacy provider wiring that served those callers while keeping unrelated finance service behavior compiling.
