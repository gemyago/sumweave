# Deployment

The chart at `deploy/helm/sumweave` deploys one immutable Sumweave
image as an HTTP app Deployment, a singleton worker Deployment, a scheduler
CronJob, a pre-install/pre-upgrade migration hook, and an optional initial-user
pre-install hook. It deliberately contains no live Argo CD `Application` or
environment secrets.

The chart accepts native Kubernetes `env` and `envFrom` structures. Top-level
`env` and `envFrom` apply only to the app, worker, and scheduler containers;
`migration.env` and `migration.envFrom` apply only to the migration Job. These
scopes are strictly separate and never inherit or merge with each other. The
optional initial-user Job likewise uses only `initialUser.env` and
`initialUser.envFrom`.

Consumers own all referenced ConfigMaps and Secrets, which must exist in the
release namespace.

Runtime workloads require a DML/query database identity; the migration Job must
receive a separate DDL-capable identity. At minimum, consumer configuration must
provide PostgreSQL DSNs for the finance application database and agent runtime,
non-colliding table prefixes, a JWT signing key, enabled finance-provider
credentials, and externally correct callback URLs. SQLite is local-development-only.

Set `initialUser.enabled` and reference a pre-existing credential Secret through
`initialUser.secret`. The Job runs after migrations with the runtime DML database
identity from `initialUser.env` or `envFrom`. It does not change an existing user.

The app Service exposes port 4501. `/health` is used for startup, liveness, and
readiness; it currently proves process health, not database readiness.

```sh
make -C deploy tools
make -C deploy lint
make -C deploy render-production
make -C deploy test
helm upgrade sumweave deploy/helm/sumweave --install --namespace sumweave --create-namespace --dry-run
```

Set `image.digest` or a `git-commit-*` tag in the deployment/config repository.
Private GHCR images require an `imagePullSecrets` entry. The optional HTTPRoute
is disabled by default; set `httpRoute.enabled` and provide a native
`httpRoute.spec` matching your gateway. Optional file-based TLS or
private-key integrations need separate secret-value support before enabling them.
