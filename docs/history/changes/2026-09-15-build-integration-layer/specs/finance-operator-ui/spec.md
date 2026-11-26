---
type: Historical record
title: "2026-09-15-build-integration-layer: spec"
description: "Historical spec record for 2026-09-15-build-integration-layer."
status: deprecated
superseded_by: /requirements/finance-operator-ui.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-15-build-integration-layer/specs/finance-operator-ui/spec.md"
    title: "Original openspec/changes/archive/2026-09-15-build-integration-layer/specs/finance-operator-ui/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

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

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
