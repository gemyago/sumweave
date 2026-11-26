# transaction-classification Specification

## Purpose
Describe tenant-owned ordered rules, safe category and additive-tag assignment,
and shared explicit/automatic classification. HTTP failures follow the later
[shared application error contract](../personal-access-tokens/spec.md#requirement-application-api-errors-are-stable-and-safe),
with the documented category-reference conflict retained as a specialized response.
The implementation gap for early create/replace rejections is recorded below.

## Requirements
### Requirement: Tenant-Owned Ordered Classification Rules
The finance module SHALL provide one active ordered classification-rule list per finance tenant and SHALL expose authenticated tenant-member operations to list, create, replace, delete, and move those rules.

#### Scenario: Tenant member manages rules
- **WHEN** an authenticated tenant member calls the tenant's classification-rule API
- **THEN** the system MUST support `GET /classification-rules`, `POST /classification-rules`, `PUT /classification-rules/{ruleId}`, `DELETE /classification-rules/{ruleId}`, and `POST /classification-rules/{ruleId}/move`
- **AND** reads and writes MUST reject rules, categories, or tags outside the selected tenant
- **AND** list results MUST be sorted by one-based position and then rule ID as a deterministic collision tie-breaker.

#### Scenario: Rule is created
- **WHEN** a tenant member creates a rule with match type `exact` or `contains`, a nonblank condition, a required visible same-tenant category, and zero or more optional visible same-tenant tags
- **THEN** the system MUST trim surrounding condition whitespace while preserving case, internal whitespace, and punctuation
- **AND** it MUST append the rule after the tenant's existing rules and return only the generated rule ID
- **AND** duplicate rule contents MUST remain allowed.

#### Scenario: Rule is edited or deleted
- **WHEN** a tenant member replaces a rule's match type, condition, category, or tags
- **THEN** the system MUST preserve that rule's position and validate the replacement fields as for creation
- **AND** deleting a rule MUST close the remaining position gap and remove its rule-tag associations without changing categories or tags already assigned to transactions.

#### Scenario: Rule is moved
- **WHEN** a tenant member moves a rule up or down
- **THEN** the system MUST swap adjacent positions in a database transaction
- **AND** a move past either edge MUST succeed as a no-op.

#### Scenario: Rule list is filtered by category
- **WHEN** a tenant member lists rules with a `categoryId` filter
- **THEN** the system MUST return only live rules in that tenant that reference that category.

#### Scenario: Rule tag sets round-trip through the API
- **WHEN** a tenant member creates or replaces a rule with valid `tagIds`
- **THEN** the system MUST persist the complete requested rule tag set atomically with the rule
- **AND** rule reads MUST include `tagIds` in stable ID order, using `[]` for category-only rules
- **AND** omitted `tagIds` or `[]` in a create or replacement request MUST mean no rule tags, with replacement clearing any previous rule tag associations.

#### Scenario: Invalid rule tags are rejected
- **WHEN** a rule request contains duplicate or blank tag IDs, explicit null `tagIds`, or tags that are missing, hidden, or owned by another tenant
- **THEN** the system MUST reject the request without changing the rule or its associations
- **AND** duplicate or blank tag IDs MUST return `400` with code `invalid_request`
- **AND** missing, hidden, or cross-tenant tags MUST return `404` with code `not_found`
- **AND** both responses MUST use the shared safe JSON envelope containing `code`, `message`, and `correlationId`, with correlation context retained in the response header.

#### Scenario: Rule ordering and filtering retain tags
- **WHEN** a tagged rule is moved or returned through category-filtered listing
- **THEN** its complete tag set MUST remain attached to the same rule
- **AND** category filtering MUST NOT change rule position or tag assignments.

#### Scenario: Explicit null rule tags are rejected
- **WHEN** a create or replacement request supplies `tagIds: null`
- **THEN** the system MUST reject the request with `400` without changing the rule or its associations
- **AND** the accepted shared error contract requires the safe `invalid_request` JSON envelope and correlation context.

Implementation gap: the current create/replace wrapper writes an empty `400`
for explicit null before generated binding. It also writes an empty `400` when
reading the request body fails for a reason other than the body-size limit.
These early responses do not yet satisfy the accepted shared envelope contract;
they do not call the rule service or publish work. Authentication remains in
front of the wrapper, and normal tenant checks remain in the service. A body-size
limit failure follows the separate `413` path. This observation does not change
the required error contract or permit mutation.

### Requirement: Classification HTTP Errors Preserve The Shared Contract
Classification HTTP operations SHALL use the shared application error envelope and status mappings while preserving tenant isolation and the documented specialized category-reference conflict.

#### Scenario: Classification request is unauthenticated or forbidden
- **WHEN** classification HTTP authentication fails or a valid credential is not permitted to call the operation
- **THEN** authentication failure MUST return `401` with code `unauthorized`, and credential-policy denial MUST return `403` with code `insufficient_permission`
- **AND** the response MUST contain only safe `code`, `message`, and `correlationId` fields and retain correlation context in the response header
- **AND** the request MUST NOT mutate rules or ledger data or publish classification work.

#### Scenario: Rule management tenant access is denied
- **WHEN** an authenticated caller attempts rule management in a tenant they have not joined
- **THEN** the API MUST return `403` with code `tenant_access_denied` in the shared safe JSON envelope
- **AND** it MUST NOT reveal that tenant's rules, mutate rules or associations, or publish classification work.

#### Scenario: Category removal is blocked by live rules
- **WHEN** a tenant member tries to remove a category referenced by a live classification rule
- **THEN** removal MUST return the specialized `409` JSON body containing code `category_referenced_by_classification_rules` and the referencing `ruleIds`
- **AND** this documented body MUST remain distinct from the common error envelope
- **AND** the category, rules, existing transaction categories, and tag assignments MUST remain unchanged.

### Requirement: Deterministic Description Matching
The classifier SHALL evaluate the loaded ordered rules from first to last and SHALL apply at most the first matching rule to each transaction.

#### Scenario: Exact condition matches
- **WHEN** a transaction description and an `exact` condition differ only by casing or surrounding whitespace
- **THEN** the exact condition MUST match the whole normalized description.

#### Scenario: Contains condition matches literally
- **WHEN** a normalized transaction description contains a normalized `contains` condition as a literal case-insensitive substring
- **THEN** the contains condition MUST match without regular expressions, wildcards, or whole-word semantics.

#### Scenario: Internal text remains significant
- **WHEN** descriptions differ in internal whitespace or punctuation
- **THEN** matching MUST preserve those differences rather than normalizing them.

#### Scenario: First visible rule wins
- **WHEN** more than one ordered rule matches a transaction
- **THEN** the classifier MUST choose the first matching rule regardless of match type
- **AND** later rule edits, deletions, or moves MUST NOT alter an already assigned transaction category or tag set.

#### Scenario: Blank description matches nothing
- **WHEN** the current user-visible transaction description is empty or whitespace-only
- **THEN** no classification rule MUST match it.

#### Scenario: Only the winning rule supplies tags
- **WHEN** several rules match an eligible transaction
- **THEN** only the first matching rule MUST supply the category and optional tags
- **AND** tags from later matching rules MUST NOT be accumulated.

### Requirement: Shared Safe Classification Selection
Automatic and explicit classification SHALL use the same finance-owned range classifier and SHALL assign a required category and optional additive tags only to currently eligible transactions.

#### Scenario: Eligible ordinary transaction is selected
- **WHEN** a transaction belongs to the selected tenant, has `effectiveAt` within the half-open requested range, has no category, is visible and booked, and has kind `regular`, `expense`, `income`, or `refund`
- **THEN** the classifier MUST consider it regardless of whether its source is manual, CSV, or provider sync.

#### Scenario: Ineligible transaction is excluded
- **WHEN** a transaction is categorized, pending, hidden or deleted, a transfer, a reconciliation, an opening balance, outside the requested range, or outside the selected tenant
- **THEN** the classifier MUST exclude it from selection
- **AND** excluded rows MUST NOT contribute to classified, unmatched, or skipped attempt counts.

#### Scenario: Eligibility changes during processing
- **WHEN** a selected transaction becomes ineligible or categorized before its update commits
- **THEN** the conditional classification write MUST leave its current category, tags, and other fields unchanged
- **AND** the attempt MUST count that selected row as skipped.

#### Scenario: Matching rule assigns a category and optional tags
- **WHEN** the first matching rule still references a visible category and only visible tags in the same tenant and the transaction remains eligible
- **THEN** the classifier MUST update only `categoryId` and `updatedAt` on the transaction row and insert missing rule tags into its tag assignments in one short database transaction
- **AND** existing tags MUST be preserved and overlapping tags MUST NOT create duplicates
- **AND** a rule with no tags MUST leave transaction tags unchanged
- **AND** the classified count MUST increase only after the write commits.

#### Scenario: Matching category is unavailable
- **WHEN** a loaded rule's category is missing, hidden, nil, or owned by another tenant before assignment
- **THEN** the classifier MUST return a finance-owned terminal failure and retain every assignment that already committed.

#### Scenario: Matching category lookup fails transiently
- **WHEN** loading a matched rule's category fails for a reason other than the persistence not-found state
- **THEN** the classifier MUST return an ordinary wrapped error rather than a finance-owned terminal failure
- **AND** existing appdispatch retry and dead-letter behavior MUST remain applicable
- **AND** every assignment that already committed and its available attempt count MUST be retained.

#### Scenario: Classification tag write fails
- **WHEN** inserting a winning rule's tags or committing its assignment fails
- **THEN** the system MUST roll back that transaction's category and tag assignment together
- **AND** it MUST retain earlier committed assignments and return an ordinary wrapped operational error
- **AND** the failed transaction MUST NOT increment the classified count.

#### Scenario: Matching tag is unavailable
- **WHEN** a loaded winning rule references a missing, hidden, or wrong-tenant tag before assignment
- **THEN** the classifier MUST return a finance-owned terminal failure with code `classification_tag_unavailable`
- **AND** it MUST leave that transaction's category and tag assignments unchanged and retain assignments that already committed.

#### Scenario: Matching tag lookup fails transiently
- **WHEN** loading a winning rule's tag fails for an operational reason rather than unavailability
- **THEN** the classifier MUST return an ordinary wrapped error for existing appdispatch retry and dead-letter handling
- **AND** it MUST leave the current transaction unchanged and retain earlier committed assignments and available attempt counts.

#### Scenario: Categorized transactions are not tagged by rules
- **WHEN** a transaction already has a category, even if it lacks tags configured on a matching rule
- **THEN** automatic and explicit classification MUST exclude it and leave its tags unchanged.

### Requirement: Bounded Attempt Execution And Diagnostics
Each classification delivery attempt SHALL load rules once, process eligible rows in bounded stable pages, and log diagnostic outcomes without persisting result payloads.

#### Scenario: Attempt begins
- **WHEN** the shared classifier starts an attempt
- **THEN** it MUST load the tenant's ordered rules and their complete tag sets once into memory
- **AND** it MUST read eligible transactions in batches initially bounded to 200 rows using a transaction-ID keyset cursor within the effective-time range.

#### Scenario: Attempt completes
- **WHEN** all selected rows have been processed
- **THEN** the service MUST log newly classified, eligible unmatched, and selected-during-processing skipped transaction counts, counting each committed classification once regardless of tag count, with tenant, message context, and elapsed time
- **AND** it MUST NOT scan excluded rows solely to count them.

#### Scenario: Attempt fails after partial progress
- **WHEN** a classification attempt fails after one or more atomic category-and-tag assignments commit
- **THEN** those writes MUST remain durable
- **AND** the service MUST log the ordinary error and available partial attempt counts.

#### Scenario: Delivery is retried
- **WHEN** appdispatch redelivers an explicit command or bank-window event
- **THEN** the new attempt MUST reload the current ordered rules and recheck current transaction state
- **AND** previously categorized transactions MUST be excluded from selection so committed categories and tags are not reapplied or overwritten
- **AND** an assignment that rolled back MUST remain eligible for a later attempt when it still satisfies selection.

### Requirement: Explicit Classification Uses Observed Durable Work
The backend application SHALL let a tenant member submit a valid classification range as a semantic command and observe it through the existing durable-jobs lifecycle.

#### Scenario: Explicit range is submitted
- **WHEN** an authenticated tenant member posts full RFC 3339 `rangeStart` and `rangeEndExclusive` timestamps with offsets to `/transactions/classify` and `rangeStart` is before `rangeEndExclusive`
- **THEN** the API MUST publish `finance.classification.explicit.v1` with tenant, range, and authenticated requester metadata
- **AND** it MUST return `202` with the immutable dispatch message ID as `jobId` without executing classification inline or creating a job row.

#### Scenario: Explicit range is invalid
- **WHEN** either range value is not a full RFC 3339 timestamp with an offset or the start is not before the exclusive end
- **THEN** the API MUST return `400` with code `invalid_request` in the shared safe JSON envelope, without publishing a command or mutating ledger data.

#### Scenario: Explicit tenant access is denied
- **WHEN** an authenticated caller submits classification for a tenant they have not joined
- **THEN** the API MUST return `403` with code `tenant_access_denied` in the shared safe JSON envelope containing `code`, `message`, and `correlationId`
- **AND** it MUST retain correlation context in the response header and leave ledger data unchanged
- **AND** it MUST NOT publish a classification command.

#### Scenario: Explicit command is delivered
- **WHEN** the worker receives the explicit command through its one job-observed consumer
- **THEN** it MUST materialize job type `finance.classification` using the message ID
- **AND** it MUST pass the command's tenant and range unchanged to the shared classifier.

#### Scenario: Explicit classification fails
- **WHEN** classification returns a finance-owned terminal failure
- **THEN** the observed job MUST use the existing sanitized failed lifecycle
- **AND** infrastructure, decoding, persistence, and unclassified failures MUST retain the existing appdispatch retry and dead-letter behavior.

### Requirement: Committed Bank Windows Trigger Automatic Classification
Every successfully committed provider-sync requested window SHALL durably trigger automatic classification for that exact tenant and ledger range independently of the overall bank-sync job.

#### Scenario: Window completion event is consumed
- **WHEN** `finance.bank-sync-window-completed.v1` is delivered to consumer group `finance.classification.v1`
- **THEN** its ordinary subscriber MUST pass the event tenant, `rangeStart`, and `rangeEndExclusive` unchanged to the shared classifier
- **AND** the automatic reaction MUST create no job projection.

#### Scenario: Later bank window fails
- **WHEN** an earlier requested window committed successfully and a later window fails
- **THEN** the earlier window's completion event and finance writes MUST remain durable
- **AND** classification of that earlier range MUST remain independently deliverable even if retry resumes beyond it.

#### Scenario: Process stops after window commit
- **WHEN** the process stops after a successful window transaction commits but before overall sync completion state is saved
- **THEN** the committed completion event MUST remain available to the classification consumer after restart.

#### Scenario: Automatic classification fails
- **WHEN** the ordinary classification subscriber fails
- **THEN** appdispatch MUST apply its normal retry and dead-letter behavior
- **AND** the committed bank imports MUST remain intact and the bank-sync job outcome MUST remain independent.

