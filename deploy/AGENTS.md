<!-- Adapted from gemyago/golang-backend-boilerplate@798f0dc9fd753481d0d698d8232ea08df44185b6. -->

## Scope

`deploy/` contains the Sumweave Helm chart only. Live Argo CD Applications,
cluster URLs, namespaces, gateways, and secrets belong in the deployment/config
repository.

## Commands

- `make -C deploy tools` installs the pinned Helm binary.
- `make -C deploy lint` lints the chart and renders default values.
- `make -C deploy test` runs chart rendering contract tests.
- Render production-like values before an install: `helm template sumweave
  deploy/helm/sumweave -f deploy/helm/sumweave/values-production.example.yaml`.

## Rules

- Never place secret values in chart values; reference existing Secret names.
- Use immutable `git-commit-*` tags or image digests for production promotion.
- Confirm the target Kubernetes context and namespace before an install.
- Keep the worker singleton and do not add persistent volumes to this chart.
