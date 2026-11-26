---
type: Requirements
title: "Backend process mode requirements"
description: "API-only startup, split consumers, scheduler ticks, and diagnostic combined startup."
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: accepted-contract
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/specs/backend-process-modes/spec.md"
    title: "Accepted backend-process-modes scenarios at the documentation checkpoint"
  - id: current-topic
    resource: "/architecture/background-processing.md"
    title: "Current topic reference and implementation limits"
---

This document preserves the accepted requirements and granular scenarios for
this capability.[^accepted-contract] See the [current topic reference](../architecture/background-processing.md)
for delivered behavior and known limitations. A requirement states a contract;
it does not record a completed test run.[^current-topic]

# backend-process-modes Specification

## Purpose
API-only startup, split consumers, scheduler ticks, and diagnostic combined startup.
## Requirements
### Requirement: Local All-In-One Backend Start Mode
The backend application SHALL provide a local all-in-one start mode that runs the normal API, jobs consumer, and scheduler components together.

#### Scenario: `start-all` is discoverable on the app binary
- **WHEN** a developer inspects the `sumweave` command tree
- **THEN** `start-all` MUST appear as a root-level command intended for local backend development

#### Scenario: `start-all` runs the full local backend shape
- **WHEN** a developer runs `sumweave start-all` after the documented schema preparation step
- **THEN** the command MUST start the HTTP server, the dedicated jobs consumer behavior, and a scheduler tick loop in one coordinated process
- **AND** the scheduler loop MUST drive the same scheduler tick behavior used by the dedicated scheduler command rather than a separate scheduling implementation

### Requirement: Dedicated Backend Command Modes Remain Available
The backend application SHALL preserve dedicated command modes for API-only, jobs consumer, and one-shot scheduler execution.

#### Scenario: API-only start stays enqueue-only
- **WHEN** a user runs `sumweave start`
- **THEN** the command MUST start only the API/server path
- **AND** it MUST NOT execute durable jobs inline or start the dedicated jobs consumer behavior automatically

#### Scenario: Dedicated command paths remain valid for split environments
- **WHEN** a user deploys or supervises backend concerns separately
- **THEN** the application MUST keep dedicated command paths for API-only startup, dedicated jobs consumer execution, and one-shot scheduler tick execution rather than requiring `start-all` in all environments

[^accepted-contract]: Preserved accepted scenario source; current delivery is described separately in the linked topic.

[^current-topic]: Current topic reference and implementation limits.
