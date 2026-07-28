# Contributing to Sumweave

## Project Setup

Please have the following tools installed:
- [direnv](https://github.com/direnv/direnv)
- [nvm](https://github.com/nvm-sh/nvm)
- Go 1.26.x tooling compatible with the repo setup

## Product Docs

Read these first:

- Product direction: [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md)
- Retained docs index: [docs/README.md](./docs/README.md)
- Domain vocabulary: [docs/domain-terminology.md](./docs/domain-terminology.md)

##  AI Frameworks

**Openspec**
```bash
npm install -D @fission-ai/openspec@latest
openspec init
```

## Typical Monorepo Tasks

If not using direnv and nvm, make sure to have go and node of a correct version as per [.nvmrc](.nvmrc) and [go.work](go.work) files.

Make sure to install deps from root of the monorepo:
```bash
# Install root deps first
npm i

# Then use nx to setup per module deps
npx nx run-many -t install-deps
```

This project uses Nx, some quick cheat-sheet:
```bash
# Run tests of specific module:
npx nx test sumweave

# Run tests bypassing cache:
npx nx test sumweave --skipNxCache

# Run all tests
npx nx run-many -t test

# Run all lint
npx nx run-many -t lint

# Run affected tasks (e.g lint):
npx nx affected --target=lint

# To see all available tasks for a specific module, use:
nx show project sumweave --json
```

To run all affected lint and tests, use `make affected-lint-test`

## Go modules specific instructions

If you want more control over deps management in go modules.

Install/Update dependencies (run from go modules): 
```sh
# Install
go mod download
go get -u tool
go install tool

# Update:
go get -u ./... && go mod tidy
```

## Run locally

The normal local workflow uses PM2 from the repository root. First migrate the
backend database, then create the PM2 process definitions:

```bash
# Run the migration from the backend module.
cd apps/sumweave
go run ./cmd/sumweave db-migrate --env local

# Return to the repository root and start the API and Vite development server.
cd ../..
npm run pm2:start
```

Backend CLI paths are relative to `apps/sumweave`. Nx and PM2 set that
working directory for the backend process; PM2 commands themselves remain
repo-root commands because `PM2_HOME` is repo-scoped.

Frontend host/port: http://localhost:5173
Backend host/port: http://localhost:4501

PM2 process names:
- `sumweave-api`
- `sumweave-ui`

Useful PM2 commands from the repo root:
```bash
npm run pm2:status
npm run pm2:restart
npm run pm2:restart:api
npm run pm2:restart:ui
npm run pm2:stop
npm run pm2:delete
```

Use `npm run pm2:logs` to inspect process output. If the backend ecosystem
command or arguments change, recreate it rather than restarting it:

```bash
pm2 delete sumweave-api
pm2 start ecosystem.config.js
```

For the optional HTTPS variant of this PM2 workflow, see
[Local HTTPS](./docs/local-https.md).

### Direct-start diagnostics only

Use direct commands only to isolate a local startup problem; they are not the
normal development workflow. From separate terminals, run the backend from
`apps/sumweave` with `go run ./cmd/sumweave start-all --env local`
and the UI from `apps/sumweave-ui` with `npm run dev`.

If the data screen still shows a browse-first availability `404` after a PM2 restart, check `npm run pm2:status`: the UI proxies `/api/v1/*` to port `4501`, and a stale non-PM2 `sumweave start` process on that port can keep PM2's backend stopped. The PM2 API entry now attempts to replace a stale `sumweave start` listener automatically on startup.

### Combined local mode

The old package-oriented combined local mode was removed. Run the backend and frontend separately, or use:
```bash
nx run-many -t dev
```
