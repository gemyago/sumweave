# Domain Terminology

- **Finance tenant**: a financial workspace with members and invitations.
- **Account**: a financial account tracked within one tenant.
- **Ledger transaction**: a recorded financial movement, including transfers.
- **Bank connection**: an authenticated external provider connection.
- **Provider evidence**: sanitized metadata retained to explain imported data.
- **Durable job**: an imperative asynchronous command with persisted lifecycle,
  result, progress, and explicit retry state.
- **Domain event**: a typed fact that already happened and can be delivered to
  multiple independent consumer groups.
- **Consumer group**: one durable reaction position for a topic; instances in
  the same group coordinate rather than each receiving a copy.
- **Dead-letter message**: an exhausted delivery retained with its original
  identity, topic, payload, and failure context for diagnosis or later replay.
- **Agent runtime**: generic sessions, profiles, providers, and workspace tooling.
