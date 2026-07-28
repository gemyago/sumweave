# Deployment

The chart at `deploy/helm/signal-foundry` deploys one immutable Signal Foundry
image as an HTTP app Deployment, a singleton worker Deployment, a scheduler
CronJob, and a pre-install/pre-upgrade migration hook. It deliberately contains
no live Argo CD `Application` or environment secrets.

Production requires existing ConfigMap and Secret names containing `APP_*`
entries. At minimum provide PostgreSQL DSNs for the finance application database
and agent runtime, non-colliding table prefixes, a JWT signing key, enabled
finance-provider credentials, and externally correct callback URLs. SQLite is
local-development-only.

The app Service exposes port 4501. `/health` is used for startup, liveness, and
readiness; it currently proves process health, not database readiness.

```sh
make -C deploy tools
make -C deploy lint
make -C deploy render-production
helm upgrade signal-foundry deploy/helm/signal-foundry --install --namespace signal-foundry --create-namespace --dry-run
```

Set `image.digest` or a `git-commit-*` tag in the deployment/config repository.
Private GHCR images require an `imagePullSecrets` entry. The optional HTTPRoute
is disabled by default and is fully values-driven. Optional file-based TLS or
private-key integrations need separate secret-value support before enabling them.
