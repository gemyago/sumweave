<!-- AGENTS.md — README for machines. Nearest file in the tree wins (hierarchical precedence). -->

## Overview

**Active.** This folder holds shared build infrastructure, currently centered on shared make fragments under **`make/`**.

## Template Origin And Boundary

This folder is template-derived support infrastructure, not part of the intended core product surface.

Treat the following as reference-only template material unless the user explicitly adopts or edits it for the real system:
- `build/make/`
- release or packaging support revived from template boilerplate

Do not infer product architecture or product commitments from this folder.

## Layout

### `build/make/`

| Path | Role |
|------|------|
| `golangci-lint.mk` | Shared repo-root pinned `golangci-lint` install/rule fragment reused by Go-module Makefiles; exports per-module cache paths under `.cache/golangci-lint/` |

Package/release pipeline assets were intentionally removed. Do not reintroduce `build/npm` or npm distribution workflows unless the user explicitly asks for them.

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

## Task Completion Protocol

Repository level task completion protocol **MUST ALWAYS** be followed. If you didn't follow it, this means task is not complete.
