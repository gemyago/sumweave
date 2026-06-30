# OpenSpec Manager Shared Worker Rules

Read this file before your role-specific `agent-*.md` file and treat both as required.

## Core contract

- The manager owns workflow sequencing, gate decisions, and independent git verification.
- You own only the role and scope assigned for this run.
- Treat `manager-status.md`, standard workflow artifacts, and the manager-named durable output file as the source of truth for prior context.
- Read every relevant `AGENTS.md` path directly before working or reviewing. Discover paths from the OpenSpec change, affected components, and `manager-status.md`.
- Persist durable run details in the referenced workflow artifact files instead of inventing new repository files.
- Return a short status plus file references and gate-relevant fields only.

## Standard artifact rules

- Do not create ad-hoc repository files to document your journey.
- Do not create scratch notes, investigation summaries, or temporary command-output files inside the repository.
- Use your final response for short notes, or use `/tmp` for scratch material that must not be committed.
- Durable workflow evidence belongs only in the manager-provided standard files: `manager-status.md`, `review-planning.md`, `review-chunk-<chunk-slug>.md`, or `review-final.md`.
- If you discover an existing ad-hoc artifact, report it and either remove it if clearly temporary or classify it as a required task artifact with a reason.
- Pending OpenSpec files and standard review or status files are part of the work and must be committed at the required commit gates.

## Status discipline

- If you update a durable workflow file, name that file explicitly in your response.
- If another sub-agent will need your output, make the relevant workflow artifact self-contained enough for handoff.
- If you cannot complete your scope cleanly, say what blocked you and which file best captures the remaining context.
