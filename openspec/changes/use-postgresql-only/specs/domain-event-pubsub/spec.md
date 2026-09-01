## RENAMED Requirements

- FROM: `### Requirement: Explicit multi-driver pub/sub schema preparation`
- TO: `### Requirement: Explicit PostgreSQL pub/sub schema preparation`

## MODIFIED Requirements

### Requirement: Explicit PostgreSQL pub/sub schema preparation

The explicit database migration command SHALL prepare the topic-aware pub/sub
message and offset schema for the configured PostgreSQL application database.

#### Scenario: Migration prepares pub/sub storage

- **WHEN** `sumweave db-migrate` runs against a supported PostgreSQL application
  database
- **THEN** it MUST prepare durable topic messages and offsets keyed by topic and
  consumer group
- **AND** later publisher and router startup MUST NOT create or alter those
  tables implicitly.
