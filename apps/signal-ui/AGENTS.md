<!-- AGENTS.md — README for machines. Nearest file in the tree wins (hierarchical precedence). -->

## Status

**Active.** Svelte 5 + Vite + TypeScript SPA. Nx project **`signal-ui`**. Root `make lint` / `make test` include this module (after `apps/signal-foundry`). Product direction: [../../docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md).

## Purpose

Browser SPA for Signal Foundry: client-side hash routing, static `dist/` build, consumes the runtime Agent API (including SSE streams).

## Template Origin And Boundary

This module is part of the intended long-term product path. Treat `apps/signal-ui/` as the real UI foundation, not throwaway scaffold.

Bootstrapped from the official Vite + Svelte TypeScript template (toolchain/bootstrap only). Do not preserve template defaults or sample patterns unless they still serve the product UI.

## Layout (module root)

```
.
├── src/
│   ├── main.ts              # app bootstrap
│   ├── App.svelte           # shell + Router / route map
│   ├── pages/               # route-level views (login, chat, providers, …)
│   ├── components/          # shared UI (nav, session list, …)
│   ├── lib/                 # shared logic (agentapi, auth, theme, data, …)
│   ├── app.css              # design tokens, theme, global defaults (layer 1)
│   └── styles/design-system.css  # ds-* primitives (layer 2)
├── public/                  # static assets → dist/
├── doc/architecture.md      # stack and integration notes
├── ui-wireframe.md          # screen structure, states, behavior
├── DESIGN.md                # visual design system
├── Makefile                 # lint, test, OpenAPI codegen
└── project.json             # Nx project
```

## Run

From the repo root:

- **`pm2 start signal-foundry-ui`** — Vite dev server on **127.0.0.1:5173** (pair with **`pm2 start signal-foundry-api`** on port **4501** for full stack).
- **`npx nx dev signal-ui`** — same dev server via Nx (runs `install-deps` first). Note: this is blocking, prefer pm2 in most cases.

In order to use the web UI you have to also make sure to start the backend server. See root AGENTS.md for more details.

From **`apps/signal-ui`**: **`npm run dev`**, **`npm run build`**, **`npm run preview`**. Install deps with **`npm ci`** (`npm install` only when changing deps). Client env: **`VITE_*`** only — see **`.env.example`**.

## Lint / test / API codegen

From **`apps/signal-ui`**:

- **`make lint`** — `npm run lint` (ESLint + `svelte-check` / TS), then **`make check-api`**.
- **`make test`** — `npm run test:run` (Vitest + coverage). Do not lower thresholds in **`vite.config.ts`**.
- **`make generate-api`** — regenerate **`src/lib/agentapi/agentapi.generated.ts`** from **`runtime/internal/agentapi/openapi.yaml`**.
- **`make check-api`** — fails if generated API types are stale (runs as part of **`make lint`**).

From the repo root: **`npx nx lint signal-ui`**, **`npx nx test signal-ui`**, or **`make affected-lint-test`** after code changes.

## Documentation

Read before changing UI; update docs in the same change when behavior shifts.

| Doc | Use for |
| --- | --- |
| [DESIGN.md](./DESIGN.md) | Styling, tokens, `ds-*` classes — **always follow** for visual changes |
| [ui-wireframe.md](./ui-wireframe.md) | Layout, routing, screen states, behavior (not styling) |
| [doc/architecture.md](./doc/architecture.md) | Stack, env, API integration, repo wiring |

## Module Rules and Conventions

Module-specific rules. Project-level rules in root [AGENTS.md](../../AGENTS.md) also apply.

The rules are:
- Update module rules when user corrects AI behavior.
- Follow [DESIGN.md](./DESIGN.md) for all UI styling changes.
- For accepted Bootstrap V2 pilot routes, prefer vanilla Bootstrap classes and native HTML/Svelte markup over custom CSS.
- Bootstrap V2 pilot remains parallel under `#/v2/*`; do not promote canonical routes here.
- Bootstrap V2 pilot pages must not add route-local `<style>` blocks or `style=` layout/styling attributes.
- Bootstrap V2 pilot custom CSS exceptions must live in shared stylesheets with a short reason comment.
- Bootstrap V2 pilot CSS exceptions are limited to shell containment, widget sizing, bridge vars, and a11y/browser fixes.
- Do not add Svelte Bootstrap wrappers or another utility CSS framework for Bootstrap V2 pilot work.
- Keep canonical routes unchanged when adding Bootstrap V2 pilot routes unless promotion is explicitly approved.
- Read [ui-wireframe.md](./ui-wireframe.md) before changing UI; update it when screens, routing, or user-visible behavior change.
- Prefer separate detail routes over dense split-pane workspaces.
- Prefer stacked summaries over oversized multi-column tables.
- Run **`make generate-api`** after editing **`runtime/internal/agentapi/openapi.yaml`**.
- Methods with more than 3 arguments (context excluded): prefer a params struct.
- Tests: do not assert TypeScript types — that is the compiler's job.
- Tests: one branch, one test; avoid excessive coverage.
- Tests: prefer per-test fixture functions over global setup (comment required if globals are unavoidable).
- Tests: use **`@faker-js/faker`** for generated sample data; exception — literals that match production (labels, routes, theme keys, domain enums).

## Important UI Verification Flow

When AI is working in bigger autonomous iterations, it must always follow the important UI verification flow in the end:
- Run relevant sub-agent to review new/updated UI using relevant UI/UX design review skill
- In case of any findings, run another sub-agent to address the findings
- Re-run UI verification agent to confirm the findings were addressed


## Using third-party packages

Before using a third-party package, ask yourself:
- Do I need to bring a third party? Is the thing big enough? - if not, implement a small custom tool/component...
- When researching the package, make sure the following:
  - It is actively maintained: enough stars ~300, commits are relatively fresh (last 6 months). New issues and pull requests are being created.
  - Would it fit our environment: is it compatible with our stack? Is it easy to integrate?
- If you are confident that the package is required: provide a summary of your investigation and justification:
  - Project is active (stars, commits, issues, pull requests)
  - Project is compatible with our stack

## Task Completion Protocol

Repository level task completion protocol **MUST ALWAYS** be followed. If you didn't follow it, this means task is not complete.

### UI Task Completion Protocol

In addition to coding task completion protocol, you must also follow the UI task completion protocol if any UI changed:
- Follow manual e2e runbook in [manual-e2e.md](../../docs/manual-e2e.md) to understand how to interact with the UI.
- Do a smoke test of the the UI changes by checking the common user flow that was changed, confirm everything is operational.
- Do a visual assessment if the UI/UX changes are visually correct and functional
- Signs of poor UI/UX experience:
  - Texts that should be usually on a single line are wrapped and hard to read
  - Elements are not aligned or overflow each other
  - Screen space is not optimally allocated creating unnecessary empty areas

If any findings discovered, resolve them and repeat the test.

Report task completion status:
- UI/UX: ✓ no issues found / discovered issues resolved
