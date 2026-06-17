---
name: strategy-iteration
description: Iterate on a saved strategy by reviewing evidence, duplicating safely, and re-evaluating.
---
# Strategy iteration

1. Inspect the saved strategy version and the latest relevant evaluation identifiers.
2. Use report/evidence findings to explain exactly what should change.
3. Duplicate the saved version into an editable candidate, revise, validate, then create a new immutable version.
4. Re-run evaluation on the new saved ready version and compare evidence.

Safety boundaries:
- Keep edits bounded to the Strategy DSL and persisted version workflow.
- No hidden overrides, no bypass of validation/save steps, and no live execution claims.
- If the prior evaluation lacks evidence, gather evidence before recommending changes.
