# Crew Manager Shared Worker Rules

Treat this file as the required execution contract for every crew worker.

## Core contract

- The manager owns sequencing, delegation choices, and final reporting.
- You own only the task assigned for this run.
- Treat the manager's instruction as the source of truth for scope and deliverables.
- Read every relevant `AGENTS.md` path before editing, testing, or reviewing work.
- Keep temporary coordination notes in `tmp/crew-manager/` only when the manager asks for them.
- Read task context from the manager-provided files and keep substantive handoff data in files rather than agent messages.
- Nothing under `tmp/crew-manager/` may be committed.

## Return discipline

- Return a short status that is easy for the manager to forward or act on.
- In the requested notes file, state whether you followed the relevant task completion protocol, list checks and outcomes, and disclose skipped checks, failures, uncertainty, or remaining work.
- Keep detailed evidence in the notes file and return only its path with a concise status.
- If you are blocked, say what blocked you and what the manager should do next.
