## Why

Enable Banking sync currently decodes successful account and transaction responses into partial typed models and then re-encodes those reduced models as provider evidence, so documented provider fields are silently lost. Account details and balances can also overwrite each other because the persisted identity does not distinguish their source-document kinds, while the terms "raw payload" and "evidence" misdescribe data that is sanitized and reconstructed from typed values.

## What Changes

- Make the Enable Banking typed client model the complete documented response shapes used by bank sync, including all documented nested fields for session accounts, account details, balances, transaction pages, and transaction items.
- Keep provider response DTOs unchanged after decoding and map them separately into normalized finance observations.
- Re-encode the typed provider DTOs into sanitized provider source snapshots; successful raw HTTP response bytes and generic raw maps will not become finance-domain or persisted data.
- Define snapshot kinds that distinguish connection, account, account-balance, and transaction documents so different documents for the same provider account cannot overwrite each other.
- Persist the latest typed snapshot for each finance subject, provider object, and snapshot kind, while retaining provider-original normalized values separately from user-edited fields.
- **BREAKING**: Replace the overlapping provider-evidence and raw-payload domain/persistence concepts with one provider-snapshot concept.
- **BREAKING**: Rename account and transaction evidence APIs and UI copy to provider source snapshots/source data rather than preserving evidence terminology or compatibility routes.
- Update the current finance terminology and provider-sync documentation to describe schema-derived provider snapshots rather than raw provider payloads.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: Require complete documented typed provider response shapes, schema-derived provider snapshots, distinct snapshot kinds, unified current-snapshot persistence, and provider-source-data API semantics.
- `finance-operator-ui`: Replace provider-evidence terminology and interactions with current provider source snapshot/source data terminology.

## Impact

- Affected finance code: `finance/internal/enablebanking/client`, `finance/internal/enablebanking`, provider sync DTOs and connectors, focused bank-sync services, provider source-data services, and finance persistence/migration models.
- Affected backend code: finance OpenAPI routes, generated route models/handlers, controllers, and finance app wiring under `apps/sumweave`.
- Affected UI code: finance API client mappings, account details, transaction details/editor source-data disclosure, tests, and `apps/sumweave-ui/ui-wireframe.md`.
- Affected storage: the new provider-snapshot table replaces the early-alpha provider evidence/raw-payload tables without data migration; GORM auto-migrate will not drop the retired tables, so upgraded database operators must remove them manually after validating the new application version.
- Affected operations: the implementation pull request and release handoff must include the exact post-upgrade table-drop statements from the design as a required operator action.
- Affected documentation/specs: finance terminology, provider-sync architecture, finance management, and finance operator UI.
- No new provider endpoint, bank provider, payment behavior, or raw-response retention mechanism is introduced.
