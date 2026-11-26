---
type: Architecture
title: "Background processing"
description: "Durable commands, independent events, optional jobs, and recovery."
status: stable
sources:
  - resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/docs/ARCHITECTURE.md"
  - resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/apps/sumweave/internal/wireup/jobs.go"
  - resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/apps/sumweave/internal/jobs/worker.go"
---

# Background processing

Appdispatch owns durable transport. Jobs add optional product visibility to a
selected consumer; they are not another execution queue. See [architecture](../ARCHITECTURE.md)
for package ownership and [local development](../operations/local-development.md)
for process commands.

## Process responsibilities

- `start`: API only; publishes commands and serves reads without executing work.
- `jobs worker`: consumes observed commands and independent ordinary event routers.
- `jobs enqueue-due`: one scheduler tick, without provider execution or job creation.
- `start-all`: combines HTTP, worker, and scheduler for startup diagnostics.

`jobs worker --once` drains observed commands first, then attempts both ordinary
classification and transfer-matching drains, combining errors. Each router uses
its configured subscription attempts and two idle polls. This bounded diagnostic
mode is not a broker-wide completion barrier; use an isolated or reseeded DB.
Normal execution cancels and joins all routers on shutdown or router failure,
then closes the shared publisher/database. Router construction alone does not
start consumption; the migration root constructs neither publisher nor router.

## Scheduling, delivery, and observed lifecycle

The scheduler first repairs any active bank connection missing a schedule by
inserting an enabled daily schedule without changing existing schedule state. It
then reads finance-owned bank-connection schedules and the finance-owned daily FX
refresh schedule. It publishes one semantic command per due occurrence, advances
the schedule, and stores the returned message ID as the future reference in one
application-database transaction. It does not use a generic schedule registry,
execute provider work, or create a job row.

Message delivery is at least once. Consumer handlers must tolerate duplicate
delivery. Routers recover panics, retry failures with the configured bounded
dispatch policy, and then publish the original message to
`app.dispatch.dead-letter.v1` with failure and source-topic metadata. A failed
dead-letter publication leaves the original message unacknowledged.

Dispatch retention is operational transport maintenance, not job retention.
Normal message rows are eligible for deletion after 7 days only when every
existing consumer-group offset for that topic has advanced beyond the row;
unacknowledged rows remain available for retry and are surfaced for operator
attention. Idempotency claims are retained for at least the same window so a
retry within the window returns the original message identity. Dead-letter
rows are retained for 30 days for diagnostics, then may be removed by the same
offset-safe maintenance process. This policy is internal to appdispatch: no
raw message or dead-letter entity is exposed through the jobs API, and it does
not define completed-job retention. No runtime cleanup is performed until an
operator maintenance command is required; workers never delete dispatch data
as part of delivery.

Each message may have at most one job-observed consumer registration. Other
consumers are ordinary appdispatch consumers; an event reaction that needs an
independent visible execution publishes a distinct semantic command. A job
projection is materialized lazily on first delivery, before the handler runs,
and uses the message ID as its job ID. Status is `queued`, `running`, `succeeded`,
or `failed`. Duplicate delivery of a terminal observed job is acknowledged
without another domain execution. Until first delivery, a request for the
known future job ID may return `404`; the UI treats that response as pending
only for an ID it just received from a dispatching workflow. Unknown or deep
linked IDs remain ordinary `404` errors.

An accepted browser trigger is requested as `operator`; an accepted access-token
trigger is requested as `integration`. Session job reads are limited to that
user's operator and integration jobs, while an access token can read only its
owner's integration jobs. Revoking a token blocks its next request but does not
cancel a command already accepted by appdispatch. The three token-writable
finance triggers accept an optional caller- and operation-scoped idempotency
key: an identical retry returns the original dispatch/job ID, while a changed
payload with that key returns `409`.

Transport failure and job failure are separate concerns. A finance service must
explicitly return a finance-owned terminal failure for a terminal domain or
provider outcome. Only the finance adapter maps that typed outcome to a handled
failure, preserving its sanitized code, summary, and details for a failed
observed job before acknowledgement. Unclassified service, malformed payload,
materialization/claim, handler-panic, and terminal-state persistence failures
follow the `appdispatch` retry/dead-letter policy; terminal-state persistence is
retried with backoff while the delivery context remains alive, and shutdown leaves
the message unacknowledged. Ordinary consumers log explicit terminal failures and
create no job state.

Only running projections whose durable claim timestamp is at least the
worker-level `staleRunningAge` old are requeued or terminally failed (the
default age is 30 minutes). Recovery conditionally matches the claim owner
and timestamp, so it cannot overwrite a newer claim or terminal transition.
Active handlers renew their claims before recovery. A canceled handler
conditionally releases its own claim while leaving the source message
unacknowledged, so a replacement worker can retry promptly. The uniform attempt
policy defaults to three attempts; handlers and individual rows do not define
competing retry limits.


## Automatic enrichment

Each committed bank-sync window publishes one typed completion event atomically
with ledger writes and its successful checkpoint. Classification and transfer
matching consume it independently in groups `finance.classification.v1` and
`finance.transfer-matching.v1`. They create no job projections and do not change
the bank-sync job outcome. Earlier windows remain actionable after a later
window fails. See [classification](../product/classification.md),
[transfer matching](../product/transfer-matching.md), and [provider sync](../finance-provider-sync-architecture.md).

## Contracts and implementation

Repository references below are outside this OKF bundle:

- [Durable jobs scenarios](../../openspec/specs/durable-jobs/spec.md)
- [Domain event scenarios](../../openspec/specs/domain-event-pubsub/spec.md)
- [Process-mode scenarios](../../openspec/specs/backend-process-modes/spec.md)
- [Transport](../../apps/sumweave/internal/appdispatch/),
  [jobs](../../apps/sumweave/internal/jobs/), and
  [worker composition](../../apps/sumweave/internal/wireup/jobs.go)
