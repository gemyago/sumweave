## MODIFIED Requirements

### Requirement: Shared Safe Classification Selection
Automatic and explicit classification SHALL use the same finance-owned range classifier and SHALL assign categories only to currently eligible transactions.

#### Scenario: Eligible ordinary transaction is selected
- **WHEN** a transaction belongs to the selected tenant, has `effectiveAt` within the half-open requested range, has no category, is visible and booked, and has kind `regular`, `expense`, `income`, or `refund`
- **THEN** the classifier MUST consider it regardless of whether its source is manual, CSV, or provider sync.

#### Scenario: Ineligible transaction is excluded
- **WHEN** a transaction is categorized, pending, hidden or deleted, a transfer, a reconciliation, an opening balance, outside the requested range, or outside the selected tenant
- **THEN** the classifier MUST exclude it from selection
- **AND** excluded rows MUST NOT contribute to classified, unmatched, or skipped attempt counts.

#### Scenario: Eligibility changes during processing
- **WHEN** a selected transaction becomes ineligible or categorized before its update commits
- **THEN** the conditional category update MUST leave its current category and other fields unchanged
- **AND** the attempt MUST count that selected row as skipped.

#### Scenario: Matching rule assigns a category
- **WHEN** the first matching rule still references a visible category in the same tenant and the transaction remains eligible
- **THEN** the classifier MUST update only `categoryId` and `updatedAt` in a short database transaction
- **AND** the classified count MUST increase only after the write commits.

#### Scenario: Matching category is unavailable
- **WHEN** a loaded rule's category is missing, hidden, nil, or owned by another tenant before assignment
- **THEN** the classifier MUST return a finance-owned terminal failure and retain every assignment that already committed.

#### Scenario: Matching category lookup fails transiently
- **WHEN** loading a matched rule's category fails for a reason other than the persistence not-found state
- **THEN** the classifier MUST return an ordinary wrapped error rather than a finance-owned terminal failure
- **AND** existing appdispatch retry and dead-letter behavior MUST remain applicable
- **AND** every assignment that already committed and its available attempt count MUST be retained.

### Requirement: Explicit Classification Uses Observed Durable Work
The backend application SHALL let a tenant member submit a valid classification range as a semantic command and observe it through the existing durable-jobs lifecycle.

#### Scenario: Explicit range is submitted
- **WHEN** an authenticated tenant member posts full RFC 3339 `rangeStart` and `rangeEndExclusive` timestamps with offsets to `/transactions/classify` and `rangeStart` is before `rangeEndExclusive`
- **THEN** the API MUST publish `finance.classification.explicit.v1` with tenant, range, and authenticated requester metadata
- **AND** it MUST return `202` with the immutable dispatch message ID as `jobId` without executing classification inline or creating a job row.

#### Scenario: Explicit range is invalid
- **WHEN** either range value is not a full RFC 3339 timestamp with an offset or the start is not before the exclusive end
- **THEN** the API MUST reject the request without publishing a command.

#### Scenario: Explicit tenant access is denied
- **WHEN** an authenticated caller submits classification for a tenant they have not joined
- **THEN** the API MUST return the standard empty `401 Unauthorized` finance response
- **AND** it MUST NOT publish a classification command.

#### Scenario: Explicit command is delivered
- **WHEN** the worker receives the explicit command through its one job-observed consumer
- **THEN** it MUST materialize job type `finance.classification` using the message ID
- **AND** it MUST pass the command's tenant and range unchanged to the shared classifier.

#### Scenario: Explicit classification fails
- **WHEN** classification returns a finance-owned terminal failure
- **THEN** the observed job MUST use the existing sanitized failed lifecycle
- **AND** infrastructure, decoding, persistence, and unclassified failures MUST retain the existing appdispatch retry and dead-letter behavior.
