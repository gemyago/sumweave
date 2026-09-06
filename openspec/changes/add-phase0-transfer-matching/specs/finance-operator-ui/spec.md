## ADDED Requirements

### Requirement: Tenant Ledger Explicit Transfer Matching
The Finance transaction ledger SHALL let the active tenant submit an inclusive local-date range for transfer matching and observe the resulting work through existing Finance job feedback.

#### Scenario: Matching range defaults to thirty local dates
- **WHEN** the tenant ledger first presents the **Match transfers** action
- **THEN** it MUST show today and the preceding 29 local calendar dates as inclusive start and end values
- **AND** it MUST allow a wider historical range and reject invalid dates or a start after the end before submission.

#### Scenario: Matching scope is tenant-wide
- **WHEN** the operator reviews the matching form before submission
- **THEN** the UI MUST explain that matching searches all accounts in the tenant and may link a partner up to 72 hours outside the selected dates
- **AND** account, search, source, sort, and current-page ledger filters MUST NOT narrow the submitted matching scope.

#### Scenario: Local dates become half-open timestamp bounds
- **WHEN** the operator submits the inclusive date range
- **THEN** the UI MUST send local start-of-day for the first date and local start-of-day after the last date as full RFC 3339 timestamps with their correct offsets
- **AND** it MUST use calendar arithmetic so daylight-saving transitions retain correct boundary offsets.

#### Scenario: Explicit matching is observed
- **WHEN** the API returns a matching `jobId`
- **THEN** the initiating flow MUST treat only that ID's pre-materialization `404` as pending, prevent duplicate submission while observing it, and show queued, running, succeeded, or failed lifecycle with a Finance job-detail link
- **AND** success MUST say “Transfer matching completed.” without claiming every transfer was found or exposing unstored counts.

#### Scenario: Terminal matching refreshes the initiating tenant
- **WHEN** the observed matching job succeeds or fails
- **THEN** the ledger MUST refresh for the tenant that initiated the run even if active tenant selection changed during observation
- **AND** failure MUST say “Transfer matching failed. Some pairs may already have been matched. Running it again preserves existing pairs.” and retain rerun availability.

#### Scenario: Manual correction remains silent
- **WHEN** the operator inspects, manually links, or unlinks transfer partners after automatic matching
- **THEN** the existing detail workflow MUST remain available without an automatic-exclusion warning, explanatory copy, or exclusion indicator.

#### Scenario: Matching UI is verified visually
- **WHEN** the ledger matching surface is ready for review
- **THEN** it MUST follow the canonical Bootstrap Finance shell and responsive behavior without route-local layout styles
- **AND** its common desktop and narrow-screen paths MUST pass the required independent UI design review and headed manual smoke loops after concrete findings are resolved.
