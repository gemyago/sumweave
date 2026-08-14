## Why

Sumweave's current `appdispatch` layer uses pub/sub terminology but only carries
durable-job execution commands through one topic and one consumer group. This
conflates command execution with domain-event fan-out and prevents independent
application consumers from reacting durably to the same event.

## What Changes

- Extend the app-owned SQL transport into durable multi-topic pub/sub with
  offsets isolated by topic and consumer group, so different consumer groups
  receive the same event independently and resume after restart.
- Add typed domain-event publishing and router/handler registration modeled on
  the proven `community-manager` Watermill design.
- Add bounded handler retry, panic recovery, and dead-letter routing without
  stopping unrelated subscriptions.
- Keep durable jobs as command processing with job lifecycle records, results,
  progress, and explicit job retry; job dispatch will use a dedicated topic and
  consumer group on the shared transport rather than becoming a domain event.
- Preserve explicit schema preparation and support both local SQLite and
  PostgreSQL transports.
- **BREAKING** Replace the early-alpha dispatch message/offset schema with a
  topic-aware shape; existing local dispatch offsets and messages are not
  migrated for compatibility.

## Capabilities

### New Capabilities

- `domain-event-pubsub`: Typed domain events, durable multi-topic publication,
  independent consumer-group delivery, restart-safe offsets, retries, panic
  recovery, and dead-letter handling.

### Modified Capabilities

- `durable-jobs`: Clarify that jobs are commands using a dedicated topic and
  consumer group on the shared transport while retaining job-specific state and
  retry semantics.

## Impact

- Affects `apps/sumweave/internal/appdispatch`, `jobs.NewModule`, the explicit
  HTTP/jobs/migration roots under `internal/wireup`, CLI lifecycle cleanup,
  application database migrations, tests, and architecture documentation.
- Reuses the existing Watermill dependencies; no new external broker or
  infrastructure service is introduced.
- Local databases must be recreated or reseeded after the topic-aware transport
  schema replaces the current early-alpha tables.
