Chunk ordering: complete sections 1 through 4 strictly in order; do not start a later section until focused tests for the current section have been written, made to fail, implemented, and passed. Within section 3, still complete 3.1 before 3.2 for review order even though 3.2's functional prerequisite is chunk 1 durable connector metadata rather than the 3.1 service cutover.

## 1. Durable Link Metadata

- [ ] 1.1 Add explicit connector identity to durable bank connection and pending-start data, and must follow TDD flow by first writing failing domain and persistence tests proving `ConnectorID` round-trips on linked bank connections, pending redirect starts persist product provider plus connector ID, and finance auto-migration prepares the new fields before implementing the minimal model and adapter changes.
- [ ] 1.2 Persist v2 redirect start results for pending links, and must follow TDD flow by first writing failing persistence tests proving a pending start can save and load the complete connector-safe `StartLinkResult` including raw payload observations and captured timestamps without plaintext credentials before implementing the JSON envelope or equivalent storage.

## 2. V2 Link Coordinator

- [ ] 2.1 Add the provider sync v2 `LinkCoordinator`, and must follow TDD flow by first writing failing coordinator tests proving product provider profiles resolve to technical connectors, unsupported provider or link-method combinations fail before connector calls, and Monobank token linking plus PKO redirect start call the expected connector methods before implementing the coordinator and narrow dependency interfaces.
- [ ] 2.2 Complete redirect finish coordination, and must follow TDD flow by first writing failing coordinator tests proving finish consumes the matching unexpired pending start for the same tenant, actor, provider, connector, and state, passes the persisted start result to the connector, restores or preserves retryability when connector finish or persistence fails, and prevents a consumed state from creating duplicate connections before implementing finish behavior.
- [ ] 2.3 Complete encrypted link persistence, and must follow TDD flow by first writing failing coordinator tests proving returned connector secrets are sealed through the existing connection-secret writer, final bank connections store only `SecretID` plus provider and connector metadata, and successful durable raw payload evidence comes only from the final token-link or redirect-finish connector result without copying pending-start start-result observations before implementing the save path.

## 3. Service Cutover And Sync Reference

- [ ] 3.1 Route existing finance service link methods through v2 link coordination, and must follow TDD flow by first updating focused service tests for `StartBankConnectionLink`, `FinishBankConnectionLink`, and `LinkTokenBankConnection` to prove public behavior stays stable, including repeated PKO redirect re-link reusing the existing tenant PKO connection instead of creating a second one, while connector/profile resolution and durable writes are v2-backed before implementing the service wiring.
- [ ] 3.2 Build provider sync v2 connection references from durable bank connections, and must follow TDD flow by first writing failing tests proving a linked Monobank connection maps to connector `monobank`, a linked PKO connection maps to connector `enable-banking`, and missing connector metadata fails before sync fetch orchestration before implementing the mapper and affected sync wiring. Dependency note: this task functionally depends on chunk 1 durable connector metadata (and the chunk 2 coordinator-owned durable shape), not on 3.1 service cutover, but it still executes after 3.1 to keep section ordering strict.

## 4. Documentation And Spec Alignment

- [ ] 4.1 Align finance provider sync documentation with v2 linking ownership, and must follow TDD flow where applicable by first identifying doc-linked expectations or terminology checks, then updating architecture wording to describe `LinkCoordinator`, durable connector identity, and encrypted link persistence before verifying the affected checks.
