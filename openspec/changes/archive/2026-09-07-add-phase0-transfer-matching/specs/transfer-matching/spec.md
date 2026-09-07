## ADDED Requirements

### Requirement: Shared Automatic Transfer Eligibility
Automatic and explicit transfer matching SHALL use one finance-owned range service and SHALL consider only loaded tenant transactions that satisfy the Phase 0 eligibility rule.

#### Scenario: Eligible transaction is loaded
- **WHEN** a visible booked transaction belongs to a visible account in the selected tenant, has kind `regular`, `expense`, `income`, or `transfer`, has a nonzero amount, has neither transfer group nor matching timestamp, and is not excluded from automatic matching
- **THEN** the matching load MUST include its ID, account ID, currency, signed minor amount, and effective timestamp regardless of manual, CSV, or provider source and regardless of category.

#### Scenario: Ineligible transaction is excluded
- **WHEN** a transaction or account is missing, hidden, deleted, or outside the tenant, or the transaction is pending, zero amount, excluded, already paired, a refund, reconciliation, opening balance, or system kind
- **THEN** the matching load MUST exclude it
- **AND** query-excluded rows MUST NOT contribute to attempt outcome counts.

#### Scenario: Existing unmatched transfer is eligible
- **WHEN** a booked visible `transfer` transaction has neither `transferGroupId` nor `transferMatchedAt` and satisfies every other eligibility condition
- **THEN** automatic matching MUST treat it like the other eligible kinds.

### Requirement: Fixed Mutual-Uniqueness Matching Rule
The transfer matcher SHALL accept a pair only when two different-account rows have exactly opposite signed minor amounts in the same currency, lie no more than 72 elapsed hours apart, and are each other's only eligible counterpart.

#### Scenario: Unique equal-and-opposite pair qualifies
- **WHEN** two eligible same-tenant transactions are on different visible accounts, use the same currency, have exact opposite nonzero minor amounts, are no more than 72 elapsed hours apart inclusive, and neither has another eligible counterpart
- **THEN** the matcher MUST accept that pair regardless of descriptions, categories, sources, provider data, or which signed leg appears first.

#### Scenario: Fixed matching boundary rejects a candidate
- **WHEN** otherwise eligible rows use different currencies, have unequal absolute amounts, occur more than 72 elapsed hours apart, share an account, or require negating the minimum signed 64-bit amount
- **THEN** they MUST NOT be automatic counterparts.

#### Scenario: Ambiguity is mutual
- **WHEN** either proposed leg has more than one eligible counterpart in its full 72-hour window
- **THEN** the matcher MUST leave the proposal unchanged rather than selecting the first or nearest candidate
- **AND** one-to-many and many-to-one cases MUST be rejected symmetrically.

#### Scenario: Decisions are deterministic
- **WHEN** the same complete loaded input is supplied in any row order
- **THEN** the matcher MUST produce the same stable deduplicated pair set
- **AND** it MUST finish all pair decisions against the unchanged loaded input before saving any pair.

### Requirement: Complete Range Evidence And Pair Scope
Each matching attempt SHALL load one complete eligible ledger slice extending 144 hours on both sides of the requested range while allowing accepted pairs only when at least one leg starts inside the original half-open range.

#### Scenario: Attempt loads complete evidence once
- **WHEN** matching runs for `rangeStart` through `rangeEndExclusive`
- **THEN** it MUST issue one uncapped tenant bulk selection for effective timestamps from `rangeStart - 144 hours` inclusive through `rangeEndExclusive + 144 hours` exclusive
- **AND** it MUST NOT page, query candidates inside the matching loop, or hide ambiguity through UI filtering or processing order.

#### Scenario: Outside-range partner can match
- **WHEN** an eligible starting leg lies inside the original range and its mutually unique partner lies outside that range but within 72 hours
- **THEN** the pair MAY be accepted.

#### Scenario: Outside evidence prevents a false unique pair
- **WHEN** a proposed leg's competing counterpart lies outside the original range but inside the extended load and that leg's 72-hour window
- **THEN** the proposal MUST remain ambiguous, including the hour-0/hour-72/hour-144 boundary case.

#### Scenario: Two outside legs cannot pair
- **WHEN** both legs of an otherwise qualifying pair lie outside the original half-open range
- **THEN** that attempt MUST NOT create the pair.

#### Scenario: Processing range is invalid
- **WHEN** either timestamp bound is zero or `rangeStart` is not before `rangeEndExclusive`
- **THEN** the shared service MUST reject the attempt before loading an unbounded ledger slice.

### Requirement: Atomic Pair State And Manual Correction
Transfer pair persistence SHALL update both existing legs together through a dedicated store, preserve unrelated ledger data, and retain manual unlink corrections as automatic-matching exclusions.

#### Scenario: Automatic pair commits
- **WHEN** an accepted automatic pair's two tenant-scoped rows exist
- **THEN** one database transaction MUST set both kinds to `transfer`, assign one new shared transfer group and matching timestamp, and update modification timestamps
- **AND** amounts, currencies, effective timestamps, descriptions, categories, tags, provider data, exclusion state, and account-balance effects MUST remain unchanged.

#### Scenario: Pair write is incomplete
- **WHEN** either expected row is missing during an automatic pair save
- **THEN** neither leg MUST change and the attempt MUST count the pair as skipped and continue
- **AND** a database error MUST roll back that pair, fail the attempt, and retain every pair committed earlier in the attempt.

#### Scenario: Manual link retains broader validation
- **WHEN** the existing manual link workflow accepts two transactions, including unequal amounts, different currencies, or an excluded leg
- **THEN** it MUST atomically write the pair fields without clearing either leg's exclusion state.

#### Scenario: Manual unlink silently excludes both legs
- **WHEN** a tenant member unlinks a valid pair
- **THEN** both legs MUST atomically return to kind `regular`, clear their transfer group and matching timestamp, preserve categories and tags, and set automatic-matching exclusion to true
- **AND** the current interaction MUST add no warning, explanation, or exclusion indicator.

#### Scenario: Ordinary save preserves pair-owned state
- **WHEN** an existing transaction is saved by user edit, provider refresh, sync apply, or another ordinary upsert
- **THEN** its persisted kind, transfer group, matching timestamp, and automatic-matching exclusion MUST remain unchanged
- **AND** any returned saved transaction MUST reflect those persisted values.

#### Scenario: Sequential attempt is repeat-safe
- **WHEN** a later attempt reloads after a pair committed or a user unlinked a pair
- **THEN** completed pairs and excluded legs MUST be absent from eligibility
- **AND** concurrent matching or edits after a load MUST have no stronger guarantee than ordinary atomic pair writes in Phase 0.

### Requirement: Matching Attempt Diagnostics
The shared matching service SHALL keep per-attempt outcome counts in memory and log them with range and dispatch context without persisting a result payload.

#### Scenario: Attempt completes
- **WHEN** matching finishes evaluating and writing its planned pairs
- **THEN** it MUST log loaded-row count, newly committed `matchedPairs`, eligible starting-row `unmatched`, eligible starting-row `ambiguous`, missing-row `skipped`, elapsed time, tenant, message ID, optional source-sync message ID, and exact range bounds
- **AND** rows outside the original range or excluded by selection MUST NOT contribute starting-row outcomes.

#### Scenario: Attempt fails after progress
- **WHEN** a matching attempt fails after one or more pair commits
- **THEN** those pairs MUST remain durable
- **AND** the service MUST log the ordinary error and available partial counts without claiming unattempted pairs as matched.

#### Scenario: Delivery is retried
- **WHEN** appdispatch redelivers matching work after failure
- **THEN** the service MUST reload the current eligible slice and recompute the attempt from current data.

### Requirement: Explicit Matching Uses Observed Durable Work
The backend application SHALL let an authenticated tenant member submit a matching range as a semantic command and observe it through the existing jobs lifecycle.

#### Scenario: Explicit range is submitted
- **WHEN** an authenticated tenant member posts offset-bearing RFC 3339 `rangeStart` and `rangeEndExclusive` values with start before end to `/transactions/match-transfers`
- **THEN** the API MUST publish `finance.transfer-matching.explicit.v1` with tenant, exact bounds, and authenticated requester metadata
- **AND** it MUST return `202` with only the immutable dispatch message ID as `jobId` without matching inline or creating a job row.

#### Scenario: Explicit submission is invalid
- **WHEN** authentication, tenant membership, timestamp parsing, nonzero bounds, or strict range ordering fails
- **THEN** the API MUST reject the request without publishing matching work.

#### Scenario: Explicit submission is repeated
- **WHEN** a tenant member submits the same valid range again after an earlier submission
- **THEN** the new user action MUST publish a fresh command and return a distinct future job ID
- **AND** redelivery of either already-published command MUST retain that command's existing identity.

#### Scenario: Explicit command is delivered
- **WHEN** the observed worker receives the command
- **THEN** it MUST lazily materialize job type `finance.transfer-matching` with the dispatch message ID and pass the tenant and exact range unchanged to the shared matcher
- **AND** only the initiating flow MAY treat that known ID's pre-materialization `404` as pending.

#### Scenario: Explicit matching fails
- **WHEN** the matching handler returns a finance-owned terminal failure
- **THEN** the observed job MUST use the existing sanitized failed lifecycle
- **AND** decoding, persistence, infrastructure, unclassified, and visibility-state failures MUST retain appdispatch retry and dead-letter behavior.

### Requirement: Committed Bank Windows Trigger Independent Matching
Every committed bank-sync-window completion event SHALL reach transfer matching through its own ordinary consumer group independently of automatic classification and the bank-sync job outcome.

#### Scenario: Window event triggers matching
- **WHEN** `finance.bank-sync-window-completed.v1` is delivered to consumer group `finance.transfer-matching.v1`
- **THEN** the handler MUST pass the event tenant, exact range, event message ID, and source sync message ID to the shared matcher without applying a connection, account, or source filter
- **AND** it MUST create no automatic matching job projection.

#### Scenario: Both enrichments receive the event
- **WHEN** one committed window event is available to the classification and matching consumer groups
- **THEN** each group MUST receive its independent delivery and neither enrichment's completion or failure may gate the other.

#### Scenario: Matching failure preserves sync progress
- **WHEN** automatic matching fails or a later provider window fails
- **THEN** earlier committed ledger writes, checkpoints, completion events, and bank-sync outcome MUST remain governed by their existing durable behavior
- **AND** matching failure MUST use ordinary dispatch retry/dead-letter handling without changing bank-sync job state.

#### Scenario: Classification and reporting stay independent
- **WHEN** classification runs before or after a qualifying transfer match
- **THEN** matching MUST preserve existing categories and classification MUST retain its existing skip behavior for transfers
- **AND** matched legs MUST remain in account balances while existing reporting excludes them from income and expenses.

### Requirement: Worker Owns Both Automatic Enrichments
The backend worker SHALL run observed commands plus independent classification and transfer-matching event routers and SHALL stop every router before shared publisher and database resources.

#### Scenario: Long-running worker executes all routers
- **WHEN** normal worker or `start-all` execution begins
- **THEN** observed commands, classification events, and matching events MUST run under one cancellation and router-error lifecycle.

#### Scenario: Bounded worker drains a committed window
- **WHEN** `jobs worker --once` processes work that emits a bank-sync-window event
- **THEN** it MUST drain observed commands first and then attempt both ordinary routers with bounded idle-poll behavior
- **AND** one ordinary drain error MUST NOT prevent attempting the other and their errors MUST be combined.

#### Scenario: Worker shuts down
- **WHEN** startup fails, cancellation occurs, or the worker closes
- **THEN** it MUST stop the observed, classification, and matching routers before closing their shared publisher or database.
