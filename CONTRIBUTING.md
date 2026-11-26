# Contributing to Sumweave

Sumweave is a personal finance tracking app. Start with the
[documentation index](docs/index.md), [product direction](docs/ARCHITECTURE.md),
and [domain vocabulary](docs/domain-terminology.md).

- [Local development](docs/operations/local-development.md) owns installation,
  PostgreSQL bootstrap, PM2, process modes, and startup troubleshooting.
- [Development and verification](docs/development/verification.md) covers Nx,
  module checks, finance smoke tests, and manual journeys.
- [Go coding guide](docs/golang-coding-guide.md) and the nearest `AGENTS.md`
  define implementation conventions; load the gopher skill before Go work.
- [Release and deployment](docs/operations/release-deployment.md) links the
  maintained host-build and Helm runbooks.

## Documentation workflow

Product and architecture documentation follows OKF 0.3 in `docs/`. Use the local
[okf-generate skill](.agents/skills/okf-generate/SKILL.md) to author or validate it
and [okf-consume](.agents/skills/okf-consume/SKILL.md) to answer from it. The skills
are copied from [official upstream](https://github.com/open-knowledge-format/open-knowledge-format/tree/main/skills);
[provenance](.agents/skills/OKF-UPSTREAM.md) records the pinned revision and license.

Update the [knowledge bundle](docs/development/documentation.md) alongside behavior
changes. Keep detailed acceptance scenarios in [requirements](docs/requirements/index.md),
linked from their current topic reference. Record accepted behavior separately
from implementation gaps and proposals. Retain useful old designs, decisions,
and reviews as [historical records](docs/history/index.md), with clear lifecycle
labels; an old task checklist does not establish current delivery.

Follow the maintenance guide for sources, claim footnotes, trust metadata,
indexes, the change log, and validation. Generic agent and Chat work is
[experimental](docs/experiments/agent-runtime.md), not the product roadmap.
