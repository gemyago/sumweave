---
type: Historical record
title: "2026-07-07-repair-enablebanking-client-contract: spec"
description: "Historical spec record for 2026-07-07-repair-enablebanking-client-contract."
status: deprecated
superseded_by: /requirements/finance-management.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-07-repair-enablebanking-client-contract/specs/finance-management/spec.md"
    title: "Original openspec/changes/archive/2026-07-07-repair-enablebanking-client-contract/specs/finance-management/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## ADDED Requirements

### Requirement: Enable Banking Client Matches Official API And App Transport
The finance module SHALL implement Enable Banking through schema-faithful typed client operations that use the backend app's configured HTTP client instance.

#### Scenario: App wiring supplies provider HTTP client
- **WHEN** the backend app composes the finance module
- **THEN** finance provider connectors MUST receive an HTTP client created by the app HTTP client factory
- **AND** Enable Banking calls MUST use that injected client for transport, timeout, logging, correlation, and telemetry behavior
- **AND** app DI wiring MUST NOT pass `http.DefaultClient` directly for normal finance provider calls

#### Scenario: Client transport uses typed JSON request sending
- **WHEN** an Enable Banking client operation sends an HTTP request
- **THEN** it MUST use a typed JSON request helper with an injected HTTP client, typed request body, typed response target, standard JSON encode/decode behavior, and standard transport/status error handling
- **AND** it MUST attach the Enable Banking JWT `Authorization` header for signed requests
- **AND** normal operations MUST NOT decode provider responses through `map[string]any`, `DoRawObject`, or `DoRawArray`

#### Scenario: Generated models match documented AIS schemas
- **WHEN** the client models request or decode the supported account-information endpoints
- **THEN** their JSON fields MUST match the official Enable Banking API names for ASPSPs, authorization start, session authorization, session data, account details, balances, and transactions
- **AND** `GET /aspsps` MUST decode the documented object response with an `aspsps` array
- **AND** session fetch MUST support documented account IDs and `accounts_data`
- **AND** balance decoding MUST use `balance_type` and `balance_amount`
- **AND** transaction requests MUST use the `transaction_status` query parameter when filtering by status
- **AND** transaction decoding MUST use documented transaction fields including `transaction_amount`, `credit_debit_indicator`, `status`, `booking_date`, `value_date`, `transaction_date`, `entry_reference`, `transaction_id`, `note`, `remittance_information`, and `continuation_key`

#### Scenario: Raw provider evidence is isolated from schema models
- **WHEN** Enable Banking response data is exposed to connector or finance mapping code
- **THEN** generated request and response structs MUST NOT expose generic raw map fields
- **AND** connector business mapping MUST use typed schema fields only
- **AND** any provider raw-payload evidence required by finance sync MUST be carried through a separate internal evidence boundary rather than through generated model `Raw` fields

#### Scenario: Unsupported or undocumented operations stay out of the generated client surface
- **WHEN** an Enable Banking endpoint or response shape is not documented by the current official API reference and is not required for the supported PKO workflow
- **THEN** the client MUST NOT keep or add that operation as part of the supported generated surface
- **AND** finance code MUST fail through bounded unsupported-path errors rather than silently falling back to raw request construction

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
