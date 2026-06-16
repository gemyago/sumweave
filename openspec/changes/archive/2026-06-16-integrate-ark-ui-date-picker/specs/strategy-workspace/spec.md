## MODIFIED Requirements

### Requirement: Strategy Workspace UI
The operator UI SHALL provide protected strategy and evaluation workspace routes for the v0 human workflow.

#### Scenario: Strategy routes are protected and navigable
- **WHEN** an authenticated operator uses the app navigation
- **THEN** the nav MUST include strategy and evaluation workspace entries alongside existing protected operator routes
- **AND** unauthenticated access to those routes MUST redirect using the existing protected route behavior

#### Scenario: Strategy editor is constrained to supported v0 fields
- **WHEN** an operator creates or edits a strategy in the UI
- **THEN** the editor MUST expose only supported moving-average-crossover fields for identity, venue/symbol/asset class/active state, timeframe, fast window, slow window, and notes
- **AND** it MUST show validation status, deterministic errors, canonical schema/kind/instrument/timeframe/parameter preview, canonical artifact hash preview, and persisted artifact hash after save

#### Scenario: Evaluation UI starts from a saved strategy version
- **WHEN** an operator starts an evaluation from the strategy workspace
- **THEN** the UI MUST submit strategy id/version, time range, quantity, optional policy hash, and note only
- **AND** it MUST NOT submit independent strategy parameters that can mismatch the artifact

#### Scenario: Evaluation range uses shared UTC picker
- **WHEN** an operator chooses the time range for an evaluation in `#/evaluations`
- **THEN** the UI MUST provide the shared UTC-aware range picker instead of requiring free-entry UTC start and end text boxes
- **AND** the picker MUST offer Last 24h, Last 7d, Last 30d, Last 90d, and Last 180d presets
- **AND** the selected range MUST resolve to visible explicit UTC `start` and `end` values before the evaluation request is submitted

#### Scenario: Evaluation range remains deterministic after preset selection
- **WHEN** an operator activates an evaluation range preset
- **THEN** the UI MUST resolve the preset once into explicit UTC `start` and `end` values
- **AND** the submitted evaluation MUST use those explicit values rather than a moving relative range expression
- **AND** invalid or empty UTC ranges MUST show inline validation and MUST NOT submit an evaluation request
