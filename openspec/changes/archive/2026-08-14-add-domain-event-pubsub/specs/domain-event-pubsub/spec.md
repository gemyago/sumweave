## ADDED Requirements

### Requirement: Typed domain-event publication

The application SHALL provide a typed domain-event publishing contract in which
each event declares its non-empty topic and is serialized for durable delivery.

#### Scenario: Publish a typed event

- **WHEN** an application component publishes a typed event with a valid topic
- **THEN** the system MUST durably store the serialized event on that topic
- **AND** the caller MUST receive an error when serialization or persistence
  fails.

#### Scenario: Reject an event without a topic

- **WHEN** an application component attempts to publish an event whose topic is
  empty
- **THEN** the system MUST reject the event without storing a message.

### Requirement: Durable topic and consumer-group isolation

The application pub/sub transport SHALL track delivery independently for each
topic and consumer group.

#### Scenario: Different groups receive the same event

- **WHEN** two consumer groups subscribe to the same topic and an event is
  published
- **THEN** each consumer group MUST receive and acknowledge its own delivery of
  that event.

#### Scenario: Instances in one group coordinate delivery

- **WHEN** multiple consumer instances subscribe to the same topic with the
  same consumer-group name
- **THEN** they MUST act as one logical consumer instead of each receiving an
  independent copy.

#### Scenario: Consumer resumes after restart

- **WHEN** a consumer group acknowledges an event, stops, and later subscribes
  again with the same name
- **THEN** it MUST resume after its acknowledged position for that topic.

#### Scenario: Topics advance independently

- **WHEN** one consumer group subscribes to multiple topics
- **THEN** acknowledgement on one topic MUST NOT advance or suppress delivery
  on another topic.

### Requirement: Typed event routing

The application SHALL allow a named router to register typed handlers for
multiple event topics and dispatch each message only to the handler registered
for its topic.

#### Scenario: Route multiple event types

- **WHEN** a router has handlers for two event types on different topics and
  both events are published
- **THEN** each handler MUST receive its decoded event type
- **AND** neither handler MUST receive the other topic's event.

#### Scenario: Reject malformed event payload

- **WHEN** a topic message cannot be decoded into its registered event type
- **THEN** the handler invocation MUST fail through the router's standard
  failure policy rather than receiving a partially decoded event.

### Requirement: Bounded failure handling and dead-letter delivery

The application event router SHALL recover handler panics, retry handler
failures a bounded number of times, and durably route exhausted messages to a
dead-letter topic.

#### Scenario: Transient handler failure succeeds on retry

- **WHEN** an event handler returns an error and then succeeds within the retry
  limit
- **THEN** the router MUST retry the same message and acknowledge it after the
  successful attempt.

#### Scenario: Handler panic follows failure policy

- **WHEN** an event handler panics
- **THEN** the router MUST recover the panic and process it through the same
  bounded retry and dead-letter policy as a returned error.

#### Scenario: Exhausted event is dead-lettered

- **WHEN** an event handler continues failing after the retry limit
- **THEN** the router MUST durably publish a dead-letter message containing the
  original message identifier, topic, payload, and failure context
- **AND** it MUST advance the failing consumer group's original-topic position
  only after the dead-letter publication succeeds.

#### Scenario: Failed event does not stop unrelated subscriptions

- **WHEN** one event exhausts its retries and is dead-lettered
- **THEN** the router MUST continue processing later messages and other
  registered topics.

### Requirement: Transaction-bound event publication

The application pub/sub transport SHALL support publishing a message within an
existing application-database transaction.

#### Scenario: Transaction commits event and state together

- **WHEN** application state and an event message are written in the same
  transaction and that transaction commits
- **THEN** both the state and event MUST become durable and visible together.

#### Scenario: Transaction rollback hides event

- **WHEN** a transaction containing an event publication rolls back
- **THEN** the event MUST NOT be visible to subscribers.

### Requirement: Explicit router lifecycle ownership

The application SHALL start each event router only through the explicit process
root that owns its reaction handlers and SHALL release router resources before
that root releases the shared application database.

#### Scenario: Router construction does not start consumption

- **WHEN** an explicit application root constructs a publisher or router
- **THEN** no event subscription MUST start until the owning process explicitly
  runs that router.

#### Scenario: Router stops cleanly with its owner

- **WHEN** an owning process cancels a running event router or closes its root
- **THEN** the router MUST stop its subscriptions and release its messaging
  resources before the shared application database is closed
- **AND** cleanup errors MUST remain observable to the owning command or
  embedder.

### Requirement: Explicit multi-driver pub/sub schema preparation

The explicit database migration command SHALL prepare the topic-aware pub/sub
message and offset schema for the configured SQLite or PostgreSQL application
database.

#### Scenario: Migration prepares pub/sub storage

- **WHEN** `sumweave db-migrate` runs against a supported application database
- **THEN** it MUST prepare durable topic messages and offsets keyed by topic and
  consumer group
- **AND** later publisher and router startup MUST NOT create or alter those
  tables implicitly.
