# Domain Terminology

- **Finance tenant**: a financial workspace with members and invitations.
- **Account**: a financial account tracked within one tenant.
- **Ledger transaction**: a recorded financial movement, including transfers.
- **Bank connection**: an authenticated external provider connection.
- **Provider snapshot**: the latest sanitized, schema-derived provider document
  for a connection, account, or transaction. It is not a raw HTTP response.
- **Provider source data**: the operator-facing label for current provider
  snapshots.
- **Semantic command**: an application-owned description of imperative work
  published to `appdispatch`; it is not itself a job.
- **Dispatch message**: the durable appdispatch representation of a command or
  event. Publication returns its immutable message ID, which is the execution
  reference.
- **Durable job**: an optional consumer-side metadata and lifecycle projection
  for selected dispatch messages. It is materialized on first delivery, uses
  the dispatch message ID as its job ID, and contains sanitized lifecycle and
  error data;
  it does not own the command payload, queue, result, progress, or retry.
- **Observed consumer**: the one consumer registration allowed to add a job
  projection for a message. Ordinary consumers execute without job rows.
- **Domain event**: a typed fact that already happened and can be delivered to
  multiple independent consumer groups through `appdispatch`. A visible event
  reaction publishes a distinct command rather than sharing a job projection.
- **Consumer group**: one durable reaction position for a topic; instances in
  the same group coordinate rather than each receiving a copy.
- **Dead-letter message**: an exhausted delivery retained with a new durable
  transport ID plus the original message ID, topic, payload, and failure context
  for diagnosis or later replay.
- **Handled business failure**: an explicitly classified finance-owned terminal
  domain or provider outcome, mapped by the app adapter and recorded as sanitized
  failed state for an observed job before acknowledgement.
- **Transport failure**: a delivery, infrastructure, panic, or unclassified
  service failure retried and eventually dead-lettered by appdispatch rather than
  finalized as a business job outcome.
- **Agent runtime**: generic sessions, profiles, providers, and workspace tooling.
