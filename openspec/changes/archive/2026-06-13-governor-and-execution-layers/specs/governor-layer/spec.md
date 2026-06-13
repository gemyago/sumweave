## ADDED Requirements

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
