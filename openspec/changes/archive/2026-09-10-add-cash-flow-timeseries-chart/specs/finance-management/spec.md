## ADDED Requirements

### Requirement: Tenant Cash-Flow Series Reporting
The finance module SHALL expose a focused tenant cash-flow series read that aggregates settled income and expense over a caller-selected timestamp range.

#### Scenario: Protected API accepts explicit range and grouping
- **WHEN** an authenticated tenant member requests `GET /api/v1/finance/tenants/{tenantId}/cash-flow-series` with full RFC 3339 `startDate` and `endDate` timestamps and `groupBy=day` or `groupBy=month`
- **THEN** the system MUST authorize membership in the selected tenant and treat the range as inclusive start and exclusive end
- **AND** the cash-flow series MUST remain separate from the finance dashboard response
- **AND** response JSON MUST use camelCase fields

#### Scenario: Invalid or excessive series requests are rejected
- **WHEN** a cash-flow series request omits a bound or grouping, supplies an invalid or reversed range, supplies an unsupported grouping, or would generate more than 366 buckets
- **THEN** the system MUST reject the request before executing the aggregation
- **AND** untrusted grouping text MUST NOT be interpolated into a SQL expression

#### Scenario: PostgreSQL returns ordered zero-filled buckets
- **WHEN** a valid cash-flow series is requested
- **THEN** a dedicated finance persistence store MUST generate consecutive `day` or `month` buckets anchored to the supplied `startDate` and ending no later than the supplied `endDate`
- **AND** PostgreSQL MUST aggregate qualifying transactions into those buckets and return them in ascending order
- **AND** buckets without qualifying transactions MUST still be returned with zero income and expense
- **AND** a generated start equal to the exclusive range end MUST NOT create a zero-width bucket
- **AND** no database schema addition or persisted aggregate MUST be required

#### Scenario: Buckets use submitted instants without a timezone parameter
- **WHEN** the client submits full timestamps whose instants represent its selected reporting boundaries
- **THEN** filtering MUST compare transaction `effective_at` values against those exact instants
- **AND** bucket generation MUST advance from the submitted start instant using PostgreSQL interval semantics
- **AND** the API MUST NOT require or infer a separate timezone

#### Scenario: Series preserves settled reporting semantics
- **WHEN** PostgreSQL calculates cash-flow contributions for a bucket
- **THEN** only booked transactions visible through visible accounts MUST contribute
- **AND** reconciliations, opening balances, hidden transactions, and matched internal transfers MUST be excluded
- **AND** unmatched transfers and ordinary income or expense MUST follow their amount signs while refunds reduce expense
- **AND** pending transactions MUST NOT contribute

#### Scenario: Series uses tenant display currency and current FX
- **WHEN** qualifying transactions use currencies other than the tenant display currency
- **THEN** the aggregation MUST use the configured persisted current FX provider rate and round each converted transaction before summing the bucket
- **AND** unavailable conversions MUST be omitted without substituting native minor units
- **AND** the response MUST mark the series incomplete and group omitted transactions by configured provider and currency pair with a distinct affected-transaction count
- **AND** the focused response MUST contain only the requested period, selected grouping, display currency, response-wide completeness, grouped missing-FX diagnostics, and bucket timestamps and amounts
