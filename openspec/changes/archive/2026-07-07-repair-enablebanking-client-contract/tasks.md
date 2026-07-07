## 1. Client Contract And Fixtures

- [x] 1.1 Add docs-derived Enable Banking client fixtures and failing tests, and must follow TDD flow by first adding tests for `GET /aspsps`, `POST /auth`, `POST /sessions`, `GET /sessions/{session_id}`, account details, balances, and transactions using official field names before changing implementation.
- [x] 1.2 Replace raw-map generated models with schema-faithful typed models, and must follow TDD flow by first proving response structs do not expose `Raw` maps and methods decode only documented fields before removing raw helpers and compatibility alias extraction.

## 2. Typed Request Sending And App Wiring

- [x] 2.1 Refactor the Enable Banking client transport to typed JSON request sending, and must follow TDD flow by first adding tests that fail while `DoRawObject`, `DoRawArray`, or method-local raw-map decoding are used for normal operations before implementing the typed sender with JWT authorization, JSON headers, standard transport/status errors, and optional separate evidence capture.
- [x] 2.2 Wire finance with the app-created HTTP client, and must follow TDD flow by first adding app DI tests proving Enable Banking calls use a client from `httpclient.ClientFactory` rather than `http.DefaultClient` before updating `financeapp` composition and any necessary finance config plumbing.

## 3. Connector Alignment

- [x] 3.1 Align Enable Banking connector mapping to corrected typed responses, and must follow TDD flow by first adding connector tests for session account IDs plus account metadata, balance selection from `balance_type`, transaction amount/direction from `transaction_amount`, stable transaction identity from `entry_reference`, and continuation from `continuation_key` before updating connector normalization.
- [x] 3.2 Keep provider evidence separate from schema models, and must follow TDD flow by first adding tests proving raw payload observations are produced without reading `Raw` fields from generated models before implementing the evidence boundary and removing any remaining generated-model raw exposure.

## 4. Manual E2E Validation

- [x] 4.1 Manually validate local bank linking with the mock Enable Banking integration after implementation, including required migrations/backend startup, initiating link, completing the redirect/callback, and confirming the linked account is visible through the existing finance status/result surface used for manual checks.
- [x] 4.2 Trigger a finance sync for the linked mock Enable Banking account and record the exact command, API path, or UI action used to start sync.
- [x] 4.3 Check whether sync succeeded using the existing sync status/result surface, and confirm expected accounts, balances, and imported transactions are present without provider errors.
- [x] 4.4 Add new transactions in the mock Enable Banking integration, re-trigger sync for the same linked account, and confirm the newly added transactions appear while previously imported transactions remain present.
- [x] 4.5 Record the manual e2e evidence in the plan/review artifacts, including environment used, link result, sync status/result observed, and before/after transaction identifiers or counts.

## 5. Final Review Follow-up

- [x] 5.1 Remove the unsupported `ListAccounts` client surface and its tests from the Enable Banking client so the exposed methods remain limited to the documented AIS endpoints used by finance.
- [x] 5.2 Update connector transaction identity selection to prefer `entry_reference` before `transaction_id`, including focused coverage for responses that contain both fields.
