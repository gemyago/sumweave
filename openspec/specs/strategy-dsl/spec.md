# strategy-dsl Specification

## Purpose
TBD - created by archiving change add-strategy-governor-v0-artifacts. Update Purpose after archive.
## Requirements
### Requirement: Strict Strategy DSL V0 Shape
The system SHALL define Strategy DSL v0 as a strict data-only definition for the existing moving-average-crossover strategy.

#### Scenario: Valid moving-average-crossover DSL is accepted
- **WHEN** a Strategy DSL v0 payload identifies the moving-average-crossover strategy, a valid canonical instrument, a valid timeframe, a positive fast window, and a larger positive slow window
- **THEN** the system MUST accept the payload and produce a canonical Strategy DSL value

#### Scenario: Unsupported strategy kind is rejected
- **WHEN** a Strategy DSL v0 payload identifies any strategy kind other than moving-average-crossover
- **THEN** the system MUST reject the payload with a validation error
- **AND** it MUST NOT produce a canonical Strategy DSL value for that payload

#### Scenario: Invalid instrument or timeframe is rejected
- **WHEN** a Strategy DSL v0 payload includes an instrument or timeframe that is not accepted by the existing domain constructors
- **THEN** the system MUST reject the payload with a validation error
- **AND** it MUST NOT produce a canonical Strategy DSL value for that payload

#### Scenario: Invalid crossover parameters are rejected
- **WHEN** a Strategy DSL v0 payload has a non-positive fast window, non-positive slow window, equal windows, or a fast window greater than the slow window
- **THEN** the system MUST reject the payload with a validation error matching the existing moving-average-crossover parameter semantics

#### Scenario: Unknown and code-like DSL fields are rejected
- **WHEN** a Strategy DSL v0 payload includes unknown fields or fields that imply arbitrary code execution such as script, source, expression, eval, function, module, import, command, prompt, or tool definitions
- **THEN** the system MUST reject the payload with a validation error
- **AND** it MUST NOT evaluate, store, preserve, or include those fields in the canonical Strategy DSL value

### Requirement: Strategy DSL Canonicalization
The system SHALL canonicalize valid Strategy DSL v0 definitions into typed canonical values.

#### Scenario: Equivalent strategy DSL payloads have same canonical value
- **WHEN** two Strategy DSL v0 payloads differ only by JSON whitespace, object field order, or non-canonical enum casing that can be normalized by existing domain constructors
- **THEN** the system MUST produce identical canonical Strategy DSL values

### Requirement: Strategy DSL Maps To Evaluation Request
The system SHALL map canonical Strategy DSL v0 definitions to the existing deterministic strategy evaluation request boundary.

#### Scenario: DSL maps with explicit evaluation range
- **WHEN** a caller maps a canonical Strategy DSL v0 definition with an explicit valid half-open time range
- **THEN** the system MUST return a `strategy.EvaluateRequest` with the canonical instrument, timeframe, moving-average-crossover strategy kind, moving-average parameters, and supplied time range

#### Scenario: Mapping rejects invalid evaluation range
- **WHEN** a caller maps a canonical Strategy DSL v0 definition with a missing or invalid half-open time range
- **THEN** the system MUST reject the mapping with a validation error
- **AND** it MUST NOT return a partial `strategy.EvaluateRequest`

