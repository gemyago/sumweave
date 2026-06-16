# High level tests (integration/e2e)

This module contains high level tests for the project, like e2e or integration tests.

## Template Origin And Boundary

This folder is template-derived test harness material unless the user explicitly promotes a test flow into the real system.

Treat the current high-level agent harness, `integration-cli`, and scenario orchestration as reference-only examples and starting points, not as mandatory long-term product QA architecture.

## Agent tests

Use this instruction when user requests to run agent e2e tests. User must do some manual setup which AI should not be concerned with. AI should assume that everything is configured and ready to run.

The user will typically say something like "run agent integration tests" or "run agent e2e tests".

### Setup check

AI must run commands below to check if the setup is correct. If anything fails, AI must not attempt to fix anything or do any investigation if it's not requested by user. It should report the error to the user.

Always run commands from module root.

```bash
# CD to module root
cd tests/agent/integration-cli

# List configured models. Lines look like: * provider/model
go run . list-models

# Use one model name from the list (provider/model, without the leading "* ") for the check.
go run . run \
  --model '<provider/model>' \
  --prompt "hello"
```

Replace `'<provider/model>'` with a real name printed by `list-models`.

### Run agent test scenarios via the master orchestrator

Once setup check succeeds, follow [./agent/scenarios/master.md](./agent/scenarios/master.md)

## Manual browser e2e

- Manual browser e2e notes live in [../docs/manual-e2e.md](../docs/manual-e2e.md).

## Task Completion Protocol

Repository level task completion protocol **MUST ALWAYS** be followed. If you didn't follow it, this means task is not complete.
