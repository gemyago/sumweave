<!-- AGENTS.md — README for machines. Nearest file in the tree wins (hierarchical precedence). -->

## Overview

**Active.** This folder holds shared lint infrastructure and the adopted host release pipeline.

## Template Origin And Boundary

This folder is template-derived support infrastructure, not part of the intended core product surface.

`build/make/` remains template-derived shared lint support. The release files in
this directory were explicitly adopted from the pinned upstream boilerplate.

Do not infer product architecture or product commitments from this folder.

## Layout

### `build/make/`

| Path | Role |
|------|------|
| `golangci-lint.mk` | Shared repo-root pinned `golangci-lint` install/rule fragment reused by Go-module Makefiles; exports per-module cache paths under `.cache/golangci-lint/` |

Build the UI and Go binaries on the host with `make -C build dist`. It produces
Linux amd64/arm64 binaries and staged platform-agent skills. Docker only packages
those outputs; it never compiles the UI or Go source. Do not reintroduce npm
distribution workflows.

## Debugging Makefiles

```bash
# The -rR allows to filter-out built-in noise

# Allows to see how make is expanding the target and its prerequisites.
make -rR -d <target>

# Allows to see the Makefile variables and their values.
make -rR -p <target>
```

## Module rules

Project-level rules in root `AGENTS.md` apply. For this module:

- Prefer shared `build/make/` changes over duplicated per-module lint wiring.
- Keep removed package/release pipeline paths out unless explicitly requested.
- For Makefile targets prefer Makefile philosophy: targets are files but not just commands.
- Run `make -C build test` when changing release scripts.
- Release and routine test tasks must not implicitly provision PostgreSQL;
  `make postgres-verify` is the explicit serial database verification lane.

## Task Completion Protocol

Repository level task completion protocol **MUST ALWAYS** be followed. If you didn't follow it, this means task is not complete.
