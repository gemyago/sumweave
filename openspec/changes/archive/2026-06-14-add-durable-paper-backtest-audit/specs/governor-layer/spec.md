## ADDED Requirements

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
