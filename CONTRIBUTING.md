# Contributing to Signal Foundry

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
npx nx test signal-foundry

# Run tests bypassing cache:
npx nx test signal-foundry --skipNxCache

# Run all tests
npx nx run-many -t test

# Run all lint
npx nx run-many -t lint

# Run affected tasks (e.g lint):
npx nx affected --target=lint

# To see all available tasks for a specific module, use:
nx show project signal-foundry --json
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

```bash
# Start backend in a separate terminal
# Install deps as per above instructions for go modules
cd apps/signal-foundry
go run ./cmd/signal-foundry start

# Start frontend in a separate terminal
cd apps/signal-ui
npm i
npm run dev

# Start everything in one terminal
nx run-many -t dev
```

Frontend host/port: http://localhost:5173
Backend host/port: http://localhost:8080

### Combined local mode

The old package-oriented combined local mode was removed. Run the backend and frontend separately, or use:
```bash
nx run-many -t dev
```
