## ADDED Requirements

### Requirement: Provider Sync V2 Persists Succeeded State Journal
The finance module SHALL persist provider sync v2 succeeded-state snapshots in an append-only journal scoped to one bank connection so sync orchestration can resume from the latest succeeded coverage.

#### Scenario: Loading the latest succeeded snapshot for a connection
- **WHEN** multiple succeeded snapshots exist in the journal for one bank connection
- **THEN** the system MUST load the newest appended snapshot for that connection
- **AND** the loaded snapshot MUST preserve the stored attempt time, success time, successful window, run or job identity, and aggregate stats fields

#### Scenario: Missing journal state for a connection
- **WHEN** a bank connection has no succeeded snapshots in the journal
- **THEN** the system MUST return no succeeded sync state for that connection
- **AND** planning the next sync session MUST treat that connection as having no prior succeeded coverage

#### Scenario: Succeeded snapshots remain append-only and connection-scoped
- **WHEN** a new succeeded snapshot is appended for one bank connection
- **THEN** earlier succeeded snapshots for that same connection MUST remain preserved rather than overwritten
- **AND** loading the latest succeeded snapshot for a different connection MUST remain unaffected
