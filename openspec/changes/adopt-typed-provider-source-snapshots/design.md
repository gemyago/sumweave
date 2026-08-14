## Context

The active finance sync path uses provider-specific clients to obtain session, account, balance, and transaction data, maps those responses into normalized finance observations, and persists additional JSON for audit/explanation. Two persisted concepts currently overlap:

- connection-scoped `RawPayload` rows; and
- account/transaction-linked `ProviderEvidence` rows exposed through finance APIs and UI.

For Enable Banking, the successful HTTP body is decoded into a typed DTO, normalized in place, and then re-encoded. The typed DTOs describe only part of the documented `AccountResource` and `Transaction` schemas, so unknown documented fields disappear. The same account ID and broad `account` scope are also used for account-details and account-balances payloads, causing one current evidence row to replace the other.

The prior `repair-enablebanking-client-contract` change intentionally removed generic raw maps from schema models but left successful raw-response capture as the likely evidence boundary. This change supersedes that remaining raw-response direction: persisted source data will be reconstructed only from supported typed provider DTOs.

Constraints include the early-alpha allowance for breaking APIs and storage, finance-owned GORM persistence, finance independence from `runtime/`, typed provider business mapping, credential redaction, and current-only provider source data rather than a user-visible history timeline.

## Goals / Non-Goals

**Goals:**

- Model every field and nested structure documented for the supported Enable Banking account-information responses used by sync.
- Preserve all documented values through typed JSON decode and encode, without promising byte-for-byte response preservation.
- Separate immutable provider DTOs from finance normalization.
- Replace ambiguous raw-payload/evidence concepts with one current provider source snapshot model.
- Distinguish account identity/details, account-balance, transaction, and connection snapshot documents.
- Keep source snapshots sanitized, tenant-authorized, and linked to the relevant finance account or transaction when applicable.
- Give account and transaction detail screens clear current provider source data without presenting it as raw or legal evidence.

**Non-Goals:**

- Capturing successful raw HTTP response bytes, unknown undocumented provider fields, JSON whitespace, or object-key order.
- Adding Enable Banking payment endpoints, transaction-details calls, or new bank providers.
- Keeping a provider snapshot history timeline; the persisted contract remains the latest document per identity.
- Preserving existing evidence/raw-payload API paths, Go names, tables, or early-alpha data.
- Treating provider snapshots as the ledger source of truth; normalized finance records and provider-original values keep their existing roles.

## Decisions

### 1. Typed provider DTOs define the persisted source-document boundary

The Enable Banking client will model the complete documented shapes used by sync: session account resources, account details, balance resources, transaction-page envelopes, transaction items, and all referenced nested schemas. Documentation-derived fixtures will contain the full documented examples available when the change is implemented.

For any provider response field documented in the supported shape, decode followed by encode MUST retain the same JSON value. Optional scalars and nested resources will use pointers or another explicit optional representation when absence must remain distinguishable from a valid zero, false, or empty value. Tests will compare JSON semantics rather than bytes.

Unknown future or undocumented fields will continue to be ignored by normal typed decoding. The client will not use generic maps, `json.RawMessage`, or successful response body bytes to retain them.

Alternative considered: persist the original successful response body. That gives greater forward compatibility but contradicts the requested typed-contract boundary and allows the stored document to outgrow the client schema without review.

### 2. Provider DTOs remain unchanged; normalization is a separate mapping

Client operations will return provider DTOs containing provider fields only. Derived conveniences currently stored in DTOs with `json:"-"`, such as normalized IDs, uppercase currency, signed minor amounts, descriptions, and effective timestamps, will move into connector mapping or dedicated normalized values.

The connector will derive finance observations from the provider DTO without mutating it. Snapshot encoding will therefore represent the typed provider document, while normalized fields can still apply Sumweave casing, trimming, sign, fallback, and fingerprint rules.

Alternative considered: keep in-place client normalization because derived fields do not normally serialize. It is rejected because some normalization currently mutates serialized provider fields, such as currency casing, making the stored document neither the provider DTO nor a clearly defined finance object.

### 3. The persisted concept is `ProviderSnapshot`

The generic domain and persistence name will be `ProviderSnapshot`; UI copy will use “Provider source data.” “Evidence” describes a possible use of the data rather than its shape and implies stronger forensic guarantees than a schema-derived reconstruction provides. “Raw payload” is incorrect because successful raw response bytes are not retained.

A snapshot contains:

- tenant and bank-connection identity;
- a finance subject: connection, account, or transaction;
- optional finance account and transaction IDs appropriate to that subject;
- a provider snapshot kind;
- the provider object ID;
- the encoded, sanitized typed document; and
- the capture timestamp.

The JSON field may remain internally byte-backed, but domain/API naming will describe it as the snapshot document or data rather than raw payload.

Alternative considered: retain `ProviderEvidence` and only correct payload generation. It avoids API renaming but leaves the central semantic ambiguity unresolved and preserves overlapping evidence/raw-payload concepts.

### 4. Snapshot kind distinguishes document shape from attachment subject

The common kinds will be:

- `connection` for a final linked/session connection document;
- `account` for an account resource from session account data or account details;
- `account_balance` for the documented balance response associated with an account; and
- `transaction` for one documented provider transaction item.

`Subject` answers which finance object exposes the snapshot. `Kind` answers which typed provider document was encoded. Account and account-balance snapshots can therefore share the same provider account ID without colliding.

Transaction-list page envelopes are transport pagination documents and will not be persisted as current source snapshots. Each imported transaction will instead receive the complete typed transaction-item snapshot. Continuation keys remain transient sync control data.

When session account data already supplies a complete account resource, the connector will snapshot that account DTO. If the connector must call account details to obtain the resource, that newer account DTO becomes the current `account` snapshot. The balance response is always separate as `account_balance`.

Alternative considered: use provider endpoint paths as kinds. Generic semantic kinds are preferred so common persistence and UI do not encode provider-specific URLs.

### 5. One latest-snapshot store replaces duplicate evidence and raw-payload stores

Finance will use a dedicated provider snapshot store rather than extending the legacy broad store. The current snapshot identity will include tenant ID, connection ID, subject, finance subject IDs, kind, and provider object ID. A later capture replaces the current document for the same identity; a different kind creates a different current snapshot.

The existing provider-evidence and raw-payload tables and services will no longer be read or written. Because the product is early alpha, implementation will introduce the finance-owned snapshot table through GORM auto-migrate and require local/test database recreation where stale tables or data matter instead of adding compatibility copying or an application-managed data migration. GORM auto-migrate does not remove retired tables, so operators upgrading a persistent deployed database must execute the explicit post-upgrade cleanup below.

Alternative considered: retain both tables, add kind to evidence, and continue storing connection raw payloads separately. That preserves duplicated content and two retention/identity systems without a current product requirement.

### 6. Sanitization remains defense in depth

Provider DTOs for source snapshots must not contain request credentials, bearer tokens, signed JWTs, private keys, or decrypted connection secrets. The generic credential-like-key sanitizer will still run immediately before persistence and again before API response serialization as defense in depth.

Non-success response bodies may remain transient inside provider error handling for bounded diagnostics, but they will not be persisted as source snapshots or returned through provider source-data APIs.

Alternative considered: remove sanitization because the typed DTOs are explicit. It is rejected because shared/provider DTOs can evolve and defense in depth is inexpensive at this boundary.

### 7. API and UI expose current provider snapshots explicitly

The protected finance API will replace `/evidence` routes with `/provider-snapshots` routes for accounts and transactions. List operations return current metadata; detail operations return the sanitized typed document. The response contract will expose `id`, `kind`, `providerObjectId`, `capturedAt`, and optional `data` rather than evidence terminology.

Account and transaction detail UI will label the disclosure “Current provider source data,” show one row per current kind/provider object, and use an explicit “Reveal source data” action. It will continue to avoid a history affordance and will explain that the document is the latest schema-derived provider snapshot, not a raw provider response.

No compatibility routes, aliases, or dual UI wording will remain.

### 8. Other connectors adopt the common snapshot boundary

Monobank will stop forwarding successful `RawJSON` into finance-domain observations and will encode its existing typed client response/item DTOs. Synthetic will encode its provider-owned typed account and transaction source structures. This change does not add provider fields or endpoints for those connectors; it only makes their output conform to the common snapshot model.

## Risks / Trade-offs

- [Enable Banking adds a documented field after implementation] → The field is ignored until the typed client and documentation-derived fixture are updated; tests pin the supported contract date and make that boundary explicit.
- [A documented optional zero/false/empty value disappears during re-encoding] → Model optionality explicitly and add full-fixture decode/encode semantic round-trip tests.
- [The rename touches domain, persistence, API, and UI at once] → Use the early-alpha breaking-change allowance, change the vertical slice atomically, and do not maintain aliases.
- [Removing page snapshots loses pagination-level debugging context] → Keep sync logs and run metadata for page counts/errors; retain per-transaction source documents, which are the data users need to explain imported records.
- [One latest snapshot loses history] → This is current behavior and current product intent; historical snapshots remain out of scope until a retention/use case is defined.
- [A provider DTO accidentally includes secret material] → Keep typed request and response models separate, sanitize at persistence and response boundaries, and test credential-like fields are not stored or returned.
- [Existing early-alpha databases retain stale tables or lack new data] → Require recreation/reseed for affected local/test environments; require the documented manual table cleanup for upgraded deployed databases; no backward-compatible data migration is promised.

## Migration Plan

1. Add complete documentation-derived provider fixtures and failing semantic round-trip tests for supported Enable Banking sync response DTOs.
2. Complete the DTO graph and separate provider response fields from connector normalization.
3. Introduce provider snapshot domain types, kinds, dedicated persistence, and current-snapshot identity tests.
4. Adapt Enable Banking, Monobank, and synthetic connectors plus bank-link/sync orchestration to emit typed snapshots.
5. Replace provider evidence/raw-payload persistence and read services with provider snapshot services in the same finance transaction boundaries.
6. Replace backend OpenAPI routes/models/controllers and regenerate route code.
7. Replace UI API mappings, terminology, account/transaction source-data disclosures, and wireframe documentation.
8. Update finance terminology, provider-sync architecture, and affected OpenSpec wording; recreate local/test databases for manual validation.
9. Copy the operator-action block below into the implementation pull request description and release handoff so deployment cannot be treated as complete without the database cleanup.

Rollback is a source revert plus database recreation. The change does not provide a data-preserving downgrade.

### Required post-upgrade operator action

GORM auto-migrate creates `finance_provider_snapshots` but does not drop tables that are no longer registered. For an upgraded persistent database, the operator MUST first confirm that the new application version is running successfully and that `finance_provider_snapshots` exists. After confirming that rollback to a legacy build is not required, the operator MUST run these statements against the Sumweave application database:

```sql
DROP TABLE IF EXISTS finance_provider_evidence;
DROP TABLE IF EXISTS finance_raw_payloads;
```

The operator MUST confirm that both retired tables are absent after executing the statements. No row copying or backup for these tables is required by this change. Fresh databases and recreated local/test databases require no manual drop.

The implementation pull request description and release notes MUST reproduce this operator action, including the exact table names and SQL statements, under a visible **Operator action required after upgrade** heading.

## Open Questions

- None. Review comments may revise the selected `ProviderSnapshot`/“Provider source data” terminology, kind vocabulary, or breaking API route names before implementation approval.
