## ADDED Requirements

### Requirement: Canonical Execution Domain
The system SHALL define canonical cross-slice execution command, order, fill, and reconciliation records that are reusable after governor approval and independent from persistence, venue payloads, AI, and HTTP API details.

#### Scenario: Execution records do not expose implementation metadata
- **WHEN** deterministic runtime code consumes execution records
- **THEN** the shared domain records MUST expose approved governor decision source, command identity, order identity, fill identity, status, quantity, price where applicable, and UTC event times without GORM tags, table names, vendor payloads, AI prompt content, or HTTP request fields

#### Scenario: Execution command identifies approved source
- **WHEN** an execution command is created
- **THEN** it MUST retain the approved governor decision and the original strategy candidate action that caused the command

#### Scenario: Execution quantities are explicit
- **WHEN** execution turns an approved decision into a command or order
- **THEN** it MUST use an explicit positive quantity supplied to the execution layer and MUST NOT infer order size from the strategy candidate action alone

### Requirement: Approval-Only Execution Admission
The system SHALL create execution commands only from governor decisions whose status is approved.

#### Scenario: Approved decision creates command
- **WHEN** execution receives an approved governor decision with a positive quantity and valid UTC request time
- **THEN** it MUST create a deterministic execution command linked to that approved decision

#### Scenario: Rejected decision is refused
- **WHEN** execution receives a rejected governor decision
- **THEN** it MUST reject command creation with a validation error and MUST NOT return a command

#### Scenario: Blocked decision is refused
- **WHEN** execution receives a blocked governor decision
- **THEN** it MUST reject command creation with a validation error and MUST NOT return a command

#### Scenario: Missing approved decision is refused
- **WHEN** execution receives no governor decision or a malformed decision that does not retain a candidate action
- **THEN** it MUST reject command creation with a validation error and MUST NOT return a command

### Requirement: Local Order And Fill Lifecycle
The system SHALL validate local execution order and fill records from execution commands without requiring live venue submission.

#### Scenario: Command creates local order record
- **WHEN** execution records an order for a valid execution command with a venue, client order identifier, positive quantity, and valid UTC event time
- **THEN** it MUST return a canonical order record linked to the command

#### Scenario: Fill creates local fill record
- **WHEN** execution records a fill for a known order with a fill identifier, positive quantity, positive price, and valid UTC event time
- **THEN** it MUST return a canonical fill record linked to the order and command

#### Scenario: Fill cannot exceed order quantity during reconciliation
- **WHEN** execution reconciles an order whose total fill quantity is greater than the order quantity
- **THEN** it MUST return a reconciliation result that marks the order as overfilled instead of silently treating it as filled

#### Scenario: Filled quantity determines reconciliation state
- **WHEN** execution reconciles an order and total fill quantity is zero, less than the order quantity, or equal to the order quantity
- **THEN** it MUST return `open`, `partially-filled`, or `filled` reconciliation state respectively

### Requirement: Deterministic Execution Behavior
The system SHALL produce stable execution records from the same approved decisions, command parameters, order inputs, and fill inputs.

#### Scenario: Repeated command creation is stable
- **WHEN** execution creates a command from the same approved decision, quantity, and request time
- **THEN** it MUST return the same command identity and command fields

#### Scenario: Reconciliation sorts fills deterministically
- **WHEN** execution reconciles an order with multiple fills
- **THEN** it MUST process fills ordered by event time ascending and fill identity ascending for ties

#### Scenario: Execution validates UTC event times
- **WHEN** execution creates commands, orders, fills, or reconciliation records
- **THEN** it MUST normalize event times to UTC and reject missing event times with a validation error

### Requirement: Execution Service Boundary
The system SHALL expose execution behavior through a runtime service boundary that depends on canonical governor approvals and local execution records rather than on upstream workflow orchestration, AI systems, or live venue mechanics.

#### Scenario: Execution consumes governor approvals
- **WHEN** execution receives work from the deterministic path
- **THEN** it MUST consume canonical approved governor decision records or a consumer-defined interface with equivalent semantics

#### Scenario: Execution does not own upstream workflow
- **WHEN** data, analytics, strategy, and governor steps must be run together
- **THEN** execution MUST NOT become the orchestrator for those upstream slices

#### Scenario: Execution stays outside AI-assisted research
- **WHEN** execution commands or records are created for the deterministic path
- **THEN** execution MUST NOT depend on AI model calls, prompts, generated explanations, or agent session state

#### Scenario: Execution v0 avoids live venue trading
- **WHEN** the initial execution layer is introduced
- **THEN** it MUST NOT require live venue credentials, wallet signing, private trading endpoints, order submission network calls, or venue-specific trading adapters

### Requirement: On-Demand Execution V0
The system SHALL provide the initial execution layer without requiring persisted execution ledgers, backend wiring, UI screens, or new external API routes.

#### Scenario: Execution does not require execution storage
- **WHEN** the initial execution service creates commands, records orders, records fills, or reconciles local records
- **THEN** it MUST complete from provided inputs without requiring an execution table, migration, event ledger, or materialized order store

#### Scenario: External API remains unchanged
- **WHEN** the execution layer is introduced
- **THEN** the system MUST NOT require a new public HTTP endpoint for execution commands or records in order to satisfy the initial capability

#### Scenario: Backend wiring is deferred until needed
- **WHEN** the initial execution runtime slice is introduced without a current backend consumer
- **THEN** the system MUST NOT require `apps/signal-foundry` dependency injection wiring in order to satisfy the initial capability
