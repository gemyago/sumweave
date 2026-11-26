# Sumweave

Sumweave's product direction is personal budgeting and finance tracking. Current
features track finances across tenants, accounts, bank connections, and CSV
imports, with a ledger, categorization, transfers, balances, and reports. Budget
planning is not yet implemented.
It is early alpha. Generic agent execution, Chat, and model-provider settings
are experimental extensions.

Start with the [documentation](docs/index.md), [product overview](docs/product/overview.md),
[architecture](docs/ARCHITECTURE.md), or [contributor guide](CONTRIBUTING.md).

For local development, install dependencies and load the repository environment
as described in [local setup](docs/operations/local-development.md), then from
the repository root run:

```sh
make postgres-bootstrap
pm2 start ecosystem.config.js
```

Open [the UI](http://localhost:5173). PM2 runs `api`, `worker`, and `ui`;
scheduling is an explicit tick. The [local guide](docs/operations/local-development.md)
contains user setup, logs, HTTPS, and troubleshooting links.
