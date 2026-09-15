## ADDED Requirements

### Requirement: Admin Provides Self-Service Access Token Management

The protected Admin area SHALL provide an Access tokens screen where every
signed-in user manages only their own personal access tokens.

#### Scenario: Access token route is protected and discoverable

- **WHEN** an authenticated user opens the Admin overview or subnavigation
- **THEN** the UI MUST link to protected `#/admin/access-tokens`
- **AND** unauthenticated access MUST follow the existing login redirect
  behavior
- **AND** the screen MUST remain self-service rather than implying a privileged
  administrator role.

#### Scenario: User creates a token

- **WHEN** a signed-in user submits a valid name, permission, and optional expiry
- **THEN** the screen MUST add the token metadata to the newest-first list
- **AND** it MUST show the complete returned token once with an explicit copy
  action and unrecoverable-secret warning.

#### Scenario: User reviews token states

- **WHEN** token metadata is loading, empty, loaded, or fails to load
- **THEN** the screen MUST show bounded loading, empty, recoverable error, active,
  expired, and revoked states
- **AND** it MUST show name, hint, permission, creation time, optional expiry,
  and derived status without a stored secret or digest.

#### Scenario: User rotates or revokes an active token

- **WHEN** a valid action is available for an active token
- **THEN** rotate and revoke MUST require confirmation explaining immediate
  invalidation
- **AND** success MUST update the metadata list
- **AND** rotation MUST show the replacement complete token once.

#### Scenario: One-time secret leaves page state

- **WHEN** the user dismisses the issued result, navigates away, or reloads
- **THEN** the complete token MUST leave UI state and MUST NOT be recoverable
  from the later list
- **AND** it MUST NOT be written to local storage, logs, URLs, analytics, or the
  clipboard without explicit copy action.

#### Scenario: Access token screen is visually verified

- **WHEN** the screen is ready for review
- **THEN** empty, loading, error, create, rotate, revoke, active, expired, and
  revoked states MUST remain keyboard-usable and responsive on desktop and
  narrow viewports
- **AND** the changed Admin flows MUST pass the repository's required visual
  review and manual UI smoke loop after concrete findings are resolved.
