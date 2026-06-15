# governor-layer Specification

## Purpose
TBD - created by archiving change governor-and-execution-layers. Update Purpose after archive.
## Requirements
### Requirement: Canonical Governor Domain
The system SHALL define canonical cross-slice governor decision records that are reusable by execution and independent from persistence, venue, AI, strategy implementation, and HTTP API details.

#### Scenario: Governor decisions do not expose implementation metadata
- **WHEN** execution or other deterministic runtime code consumes governor output
- **THEN** the shared domain records MUST expose candidate action, decision status, reason, and UTC decision time without GORM tags, table names, venue payloads, AI prompt content, or HTTP request fields

#### Scenario: Governor decision status is explicit
- **WHEN** the governor evaluates a strategy candidate action
- **THEN** the resulting decision MUST classify the action as `approved`, `rejected`, or `blocked` with a stable reason value

#### Scenario: Governor decision preserves strategy action identity
- **WHEN** the governor returns a decision for a candidate action
- **THEN** the decision MUST retain the original canonical strategy identity, action kind, candidate decision time, candidate input range, and candidate quality

### Requirement: Deterministic Governor Evaluation
The system SHALL evaluate strategy candidate actions using explicit policy inputs with stable ordering and deterministic results.

#### Scenario: Repeated evaluation is stable
- **WHEN** the governor evaluates the same ordered candidate actions with the same policy
- **THEN** it MUST return the same ordered decisions with the same statuses, reasons, decision times, and source candidate actions

#### Scenario: Decision ordering follows candidate ordering
- **WHEN** the governor evaluates multiple candidate actions
- **THEN** it MUST return decisions ordered by the source candidate actions' decision time ascending

#### Scenario: Invalid policy is rejected
- **WHEN** a caller evaluates candidate actions with an unsupported minimum quality, empty allowed action set, or negative maximum approved action count
- **THEN** the governor MUST reject the request with a validation error and MUST NOT return partial decisions

### Requirement: Initial Policy Rules
The system SHALL support initial deterministic policy rules for allowed action kinds, minimum acceptable candidate quality, and maximum approved actions per evaluation.

#### Scenario: Eligible action is approved
- **WHEN** a candidate action uses an allowed action kind, meets or exceeds the minimum quality, and approval capacity remains
- **THEN** the governor MUST return an `approved` decision for that action

#### Scenario: Disallowed action kind is rejected
- **WHEN** a candidate action uses an action kind that is not present in the allowed action set
- **THEN** the governor MUST return a `rejected` decision with a reason identifying the disallowed action kind

#### Scenario: Candidate below minimum quality is rejected
- **WHEN** a candidate action quality is lower than the policy minimum quality
- **THEN** the governor MUST return a `rejected` decision with a reason identifying the quality policy failure

#### Scenario: Approval limit blocks otherwise eligible action
- **WHEN** a candidate action is otherwise eligible but the maximum approved action count for the evaluation has already been reached
- **THEN** the governor MUST return a `blocked` decision with a reason identifying the approval limit

### Requirement: Governor Service Boundary
The system SHALL expose governor evaluation through a runtime service boundary that depends on canonical strategy candidate actions rather than on venues, AI systems, execution side effects, or persistence.

#### Scenario: Governor consumes strategy candidate actions
- **WHEN** the governor evaluates strategy output
- **THEN** it MUST consume canonical `domain.CandidateAction` records or a consumer-defined interface with equivalent semantics

#### Scenario: Governor stays outside venue mechanics
- **WHEN** the governor evaluates actions for venue-derived market data
- **THEN** it MUST NOT depend on vendor payloads, symbol mapping rules, venue HTTP clients, signing keys, wallet credentials, or live venue network access

#### Scenario: Governor stays outside AI-assisted research
- **WHEN** governor decisions are evaluated for the deterministic path
- **THEN** the governor MUST NOT depend on AI model calls, prompts, generated explanations, or agent session state

#### Scenario: Governor emits no execution side effects
- **WHEN** the governor evaluates candidate actions
- **THEN** it MUST NOT place orders, mutate venue state, record fills, or require execution-side dependencies in order to complete evaluation

### Requirement: On-Demand Governor V0
The system SHALL provide the initial governor layer without requiring persisted governor outputs, backend wiring, UI screens, or new external API routes.

#### Scenario: Governor evaluation does not require governor storage
- **WHEN** the initial governor service evaluates candidate actions
- **THEN** the evaluation MUST complete from the provided candidate actions and policy without requiring a governor table, migration, backfill job, or materialized decision store

#### Scenario: External API remains unchanged
- **WHEN** the governor layer is introduced
- **THEN** the system MUST NOT require a new public HTTP endpoint for governor evaluation in order to satisfy the initial capability

#### Scenario: Backend wiring is deferred until needed
- **WHEN** the initial governor runtime slice is introduced without a current backend consumer
- **THEN** the system MUST NOT require `apps/signal-foundry` dependency injection wiring in order to satisfy the initial capability

### Requirement: Intent-Based Paper And Backtest Governor Input
The system SHALL support governor evaluation from explicit paper/backtest order intent context without overloading strategy candidate actions with sizing or order semantics.

#### Scenario: Governor input carries explicit intent context
- **WHEN** the governor evaluates a paper or backtest intent
- **THEN** the request MUST explicitly include mode, source strategy identity and version or artifact reference when available, candidate action or order intent source, venue, instrument, requested action kind or side, requested notional and/or quantity, limit price when available, candidate or data quality, current strategy exposure, current instrument exposure, and governor policy id/version/hash
- **AND** the governor MUST NOT infer sizing, price, venue, mode, or exposure from `domain.CandidateAction` alone

#### Scenario: Invalid intent is rejected or blocked deterministically
- **WHEN** the governor receives an intent with missing required mode, scope, strategy reference, side/action kind, quality, quantity/notional, or policy reference fields
- **THEN** it MUST return or fail with a deterministic `INVALID_INTENT` reason without producing partial approvals

#### Scenario: Reason-code strings are canonical
- **WHEN** governor reason codes are returned, persisted, linked from audit or execution records, or included in evaluation evidence
- **THEN** the persisted reason-code string MUST be exactly one of `OK`, `MODE_NOT_ALLOWED`, `VENUE_NOT_ALLOWED`, `INSTRUMENT_NOT_ALLOWED`, `STRATEGY_NOT_ALLOWED`, `ACTION_KIND_NOT_ALLOWED`, `DATA_QUALITY_TOO_LOW`, `KILL_SWITCH_ACTIVE`, `ORDER_NOTIONAL_EXCEEDS_LIMIT`, `STRATEGY_EXPOSURE_EXCEEDS_LIMIT`, `INSTRUMENT_EXPOSURE_EXCEEDS_LIMIT`, `APPROVAL_LIMIT_REACHED`, or `INVALID_INTENT`
- **AND** the system MUST NOT persist lower-kebab, lowercase, localized, or display-label variants for these governor reason codes

#### Scenario: Live mode remains unsupported
- **WHEN** the governor receives an intent whose mode is `live`
- **THEN** it MUST reject or block the intent with `MODE_NOT_ALLOWED`
- **AND** it MUST NOT enable live venue submission, wallet signing, private venue API calls, or execution side effects

### Requirement: Paper Safety Policy Checks
The system SHALL enforce deterministic paper/backtest safety checks before execution admission.

#### Scenario: Valid paper intent is approved
- **WHEN** a paper or backtest intent satisfies allowed mode, venue, instrument, strategy, action kind, quality, kill-switch, order notional, strategy exposure, instrument exposure, and approval-count rules
- **THEN** the governor MUST return an `approved` decision with stable reason `OK`

#### Scenario: Scope allowlists are enforced
- **WHEN** an otherwise valid intent uses a venue, instrument, or strategy that is not allowed by policy
- **THEN** the governor MUST reject or block it with `VENUE_NOT_ALLOWED`, `INSTRUMENT_NOT_ALLOWED`, or `STRATEGY_NOT_ALLOWED` respectively

#### Scenario: Action and data quality rules are enforced
- **WHEN** an otherwise valid intent uses a disallowed action kind or side, or its quality is below policy minimum
- **THEN** the governor MUST reject or block it with `ACTION_KIND_NOT_ALLOWED` or `DATA_QUALITY_TOO_LOW` respectively

#### Scenario: Kill switch blocks new risk
- **WHEN** policy marks new risk as blocked or kill switch active
- **THEN** the governor MUST block new-risk intents with `KILL_SWITCH_ACTIVE`

#### Scenario: Notional and exposure caps are enforced
- **WHEN** an intent exceeds maximum order notional, projected strategy exposure, or projected instrument exposure
- **THEN** the governor MUST reject or block it with `ORDER_NOTIONAL_EXCEEDS_LIMIT`, `STRATEGY_EXPOSURE_EXCEEDS_LIMIT`, or `INSTRUMENT_EXPOSURE_EXCEEDS_LIMIT` respectively

#### Scenario: Approval count still applies
- **WHEN** an otherwise valid intent would exceed the policy maximum approved count for the evaluation
- **THEN** the governor MUST return a `blocked` decision with `APPROVAL_LIMIT_REACHED`

### Requirement: Expanded Governor Determinism
The system SHALL keep expanded governor evaluation deterministic and compatible with existing simple evaluation behavior where that behavior remains in use.

#### Scenario: Repeated intent evaluation is stable
- **WHEN** the governor evaluates the same ordered intents with the same policy and exposure inputs more than once
- **THEN** it MUST return the same ordered decisions, statuses, reason codes, and decision timestamps

#### Scenario: Existing candidate-action evaluation remains testable
- **WHEN** existing callers still evaluate candidate actions through the simple governor path
- **THEN** compatibility helpers or updated tests MUST preserve the documented action-kind, minimum-quality, and approval-count behavior until those callers are migrated to intents

