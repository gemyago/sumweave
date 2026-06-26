## MODIFIED Requirements

### Requirement: Finance Sync, Secrets, And Imports
The finance module SHALL support secure provider linking plus explicit async sync/import workflows.

#### Scenario: Provider sync v2 lets target-window policy interpret the latest loaded state
- **WHEN** provider sync v2 decides the next target window
- **THEN** target-window planning MUST consume the latest loaded sync state directly
- **AND** orchestration MUST NOT construct a synthetic future state by copying prior success fields into the current attempt before any chunk is known
- **AND** concrete attempt state rows MUST be created only when an exact chunk window is being executed

#### Scenario: Provider sync v2 uses a wider initial backfill when no prior state exists
- **WHEN** provider sync v2 decides the next target window for a connection with no prior state
- **THEN** the target window MUST start 3 years before the planning time
- **AND** the target window MUST end at the planning time

#### Scenario: Provider sync v2 refreshes the recent 30-day window when prior state is recent
- **WHEN** provider sync v2 decides the next target window and the derived prior checkpoint is within the last 30 days
- **THEN** the target window MUST start 30 days before the planning time
- **AND** the target window MUST end at the planning time

#### Scenario: Provider sync v2 catches up continuously from an older checkpoint
- **WHEN** provider sync v2 decides the next target window and the derived prior checkpoint is older than 30 days
- **THEN** the target window MUST start at that prior checkpoint
- **AND** the target window MUST end at the planning time

#### Scenario: Provider sync v2 derives the prior checkpoint from latest-state outcome
- **WHEN** provider sync v2 interprets the latest loaded sync state for target-window planning
- **THEN** `state.Window.End` MUST be the prior checkpoint when `SucceededAt` is populated
- **AND** `state.Window.Start` MUST be the prior checkpoint when `SucceededAt` is absent

#### Scenario: Provider sync v2 retries the latest failed attempt window through checkpoint derivation
- **WHEN** the latest loaded sync state has `SucceededAt` absent
- **THEN** target-window planning MUST derive the prior checkpoint from `state.Window.Start`
- **AND** it MUST then apply the same 30-day recent-vs-older-checkpoint rule used for other prior checkpoints

#### Scenario: Provider sync v2 rejects invalid latest-state windows during planning
- **WHEN** the latest loaded sync state has a zero or inverted `Window`
- **THEN** target-window planning MUST fail with a bounded error instead of silently fabricating a target window
