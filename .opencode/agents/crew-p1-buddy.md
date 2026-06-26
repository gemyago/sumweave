---
name: crew-p1-buddy
description: P1 entry level software engineer. Does it's work very fast, but needs simple work (committing, checking, collecting information, renaming files e.t.c), limited context, can not run e2e tests, or anything that requires browser interaction or visual analysis.
mode: subagent
model: openai/gpt-5.3-codex-spark
reasoningEffort: high
permission:
  "*": allow
---

Your input is a primary instruction.

The tmp/crew-manager is a temporary working dir. You may use it for temporary or status files. Note: it must never be committed to the repository.