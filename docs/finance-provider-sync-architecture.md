# Finance Provider Sync Architecture

This document explains the finance provider sync flow at a high level.

## Terms

- Provider: the bank brand or institution the user connects to, such as PKO or monobank.
- Connector: the technical integration used to talk to a provider, such as Enable Banking.
- Connection: one linked bank access record for one tenant.
- Observation: normalized data reported by the provider before it becomes ledger data.
- Target window: the overall coverage range the sync session wants to bring up to date.
- Requested window: the time range one window sync execution asks the provider to return.
- Chunk window: one requested window produced when a larger target window is split.
- Candidate window: a wider persisted lookup range used to catch the same transaction when the provider later shifts its timestamp or status.
- Diff plan: the write-free plan describing what should be created or updated.
- Diff planner: the pure component that builds the diff plan for one requested sync window.
- Sync orchestrator: the component that loads the latest succeeded sync state, chooses the target window, splits it into chunk windows, and coordinates per-window execution.
- Window sync executor: the component that executes one requested sync window end to end.
- Provider-original fields: the last known raw provider values stored next to a transaction.
- Sync state: one succeeded progress snapshot for one connection.
- Sync state journal: the append-only history of succeeded sync state snapshots for one connection.
- Latest succeeded sync state: the newest succeeded snapshot loaded to decide the next target window.

## Purpose

Provider sync keeps linked bank connections up to date without treating provider
data as automatically correct ledger data.

The main idea is:

1. Fetch what the provider currently reports.
2. Compare it with what we already know.
3. Decide what should change.
4. Apply those changes conservatively.

## Main Flow

### 1. A connection is linked

A user links a bank connection such as monobank or PKO.

- The product-level provider is what the user sees.
- The technical connector is how we talk to the provider.
- For example, PKO is the provider, while Enable Banking is the connector.

### 2. A sync is requested

Sync can be started manually or by a scheduled job.

The request is scoped to one bank connection.

### 3. The sync orchestrator plans the session

The sync orchestrator loads the latest succeeded sync state for the connection.

That detail matters because sync progress is modeled as a journal, not as one
mutable state row that gets overwritten on every attempt.

In practice:

- each succeeded chunk appends the next succeeded sync state snapshot
- failed attempts do not replace the latest succeeded snapshot
- resume starts from the latest succeeded coverage, not from the start of the failed session

It then decides the target window for this sync session.

The intended policy is:

- sync at least the last 30 days ending at the current time
- if the last successful window ended earlier than that, extend the target window backward to catch up from `lastSuccessfulWindow.End`
- if that target window is longer than 30 days, split it into chunk windows of at most 30 days
- execute chunk windows oldest first

This keeps the rolling refresh behavior separate from the one-window execution logic.

### 4. The window sync executor fetches provider observations

For each chunk window, the window sync executor uses the connector to fetch normalized observations, not final ledger records.

That includes:

- provider accounts
- provider balances
- provider transactions
- raw provider payloads

This keeps provider data separate from the user-facing finance ledger.

### 5. The system loads the existing window

Before changing anything, the system loads the existing persisted data that may
match the incoming provider observations.

This lookup window can be a bit wider than the requested sync window.

We need that because providers do not always report the same transaction with
exactly the same timestamp or status over time. A pending transaction may later
become booked, or a provider may slightly shift the effective time after
settlement. If we only looked inside the exact requested window, we could miss
the earlier stored record and create an avoidable duplicate.

### 6. The system builds a diff plan

The next step is to create a plan that says what should happen.

This step is pure planning:

- no provider calls
- no persistence writes
- no hidden side effects

The diff planner decides whether each provider transaction should:

- update an existing transaction
- create a new transaction

### 7. Matching stays conservative

Strong matches are updated.

Weak or ambiguous matches create a new transaction instead of merging into an
existing one. This is intentional: duplicates are safer than silently merging
the wrong financial event.

### 8. User edits are preserved

When a synced transaction already exists, the system refreshes the stored
provider-original values from the new provider observation.

But user-facing fields are only overwritten when they still match the previous
provider-original values. If a user changed a description, amount, or date, the
sync should preserve that edit instead of erasing it.

### 9. The plan is applied atomically

After planning, the system applies the intended changes to persistence.

At a high level, this writes:

- updated or newly created transactions
- provider match records
- account and balance observations
- raw payload observations
- sync run and sync state metadata

### 10. Sync state is updated

After each succeeded chunk window, the system appends the next succeeded sync state
snapshot for the connection.

Conceptually, the journal for one connection evolves like this:

```text
state0 -> state1 -> state2 -> state3
```

Where each next state extends the known succeeded coverage.

If a later chunk fails, the latest succeeded state stays unchanged, so the next
session resumes from that point.

That snapshot includes:

- last attempt
- last success
- last successful window
- last run or job reference
- stats

Failed sessions do not replace the latest succeeded sync state snapshot. Resume
starts from the latest succeeded window, while failures should be traced through
run records and logs.

## Design Principles

- Keep provider data separate from ledger data.
- Plan before writing.
- Prefer explicit matches over clever guesses.
- Preserve user edits.
- Keep a succeeded sync state journal per connection.
- Load the latest succeeded snapshot before planning the next session.
- Append succeeded progress; do not overwrite it on failed attempts.
- Keep provider-specific transport details behind connectors.

## In Short

Conceptually, provider sync is:

```text
connection -> load latest succeeded sync state -> choose target window -> split into chunk windows -> execute each requested window -> load existing window -> plan diff -> apply changes -> append next succeeded sync state
```

The important part is not just fetching bank data. The important part is making
sync explainable, conservative, and safe for user-managed ledger data.
