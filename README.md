# Signal Foundry

[![Build](https://github.com/gemyago/signal-foundry/actions/workflows/build-flow.yml/badge.svg)](https://github.com/gemyago/signal-foundry/actions/workflows/build-flow.yml)

Signal Foundry is an early-stage trading platform project.

The current product direction is defined in [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md). Historical template and implementation-planning docs have been removed to keep the repository oriented around that architecture.

## Repository Shape

- `runtime/` — core Go runtime foundation
- `apps/signal-foundry/` — Go backend application
- `apps/signal-ui/` — operator UI

Everything else should be treated as support or reference material unless explicitly adopted into product scope.

## Start Here

- Product direction: [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md)
- Retained project docs: [docs/README.md](./docs/README.md)
- Contributor workflow: [CONTRIBUTING.md](./CONTRIBUTING.md)
- Agent instructions: [AGENTS.md](./AGENTS.md)
