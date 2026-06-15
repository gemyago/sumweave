## ADDED Requirements

### Requirement: Durable Paper Backtest Audit Linkage
The system SHALL link the thin paper/backtest orchestration flow to audit, intent, governor, execution ledger, snapshot, and report records without moving slice-owned behavior into the flow.

#### Scenario: Flow creates trace before intent and governor
- **WHEN** the durable paper/backtest flow processes a strategy action that may produce an execution intent
- **THEN** it MUST persist or request persistence of a decision trace before creating the order intent
- **AND** it MUST pass the resulting intent context to governor evaluation rather than adding sizing and order semantics to the candidate action

#### Scenario: Flow records downstream references
- **WHEN** governor evaluation and execution ledger steps complete for a traced intent
- **THEN** the flow MUST update or append audit references sufficient to navigate from decision trace to order intent, governor decision, execution command, order, fill, and snapshot/report records when those downstream records exist

#### Scenario: Flow remains a thin coordinator
- **WHEN** durable audit, execution ledger, paper state, and evaluation records are added
- **THEN** `runtime/flows` MUST coordinate package services and preserve Data -> Analytics -> Strategy -> Governor -> Execution ordering
- **AND** it MUST NOT duplicate governor checks, execution ledger persistence, limit-fill simulation, position math, or backtest/evaluation persistence internals

#### Scenario: Durable flow avoids unrelated product surfaces
- **WHEN** the durable linkage is introduced
- **THEN** it MUST NOT require new HTTP routes, UI screens, scheduled jobs, live venue credentials, wallet signing, private trading endpoint calls, or AI model calls
