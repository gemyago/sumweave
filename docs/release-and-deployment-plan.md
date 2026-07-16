# Release And Deployment Plan

## Goal

Build frontend and backend release artifacts on the host or CI runner, package
only prepared artifacts in Docker, publish one multi-platform image, and deploy
the same image into separate production workloads with Helm and Argo CD.

The implementation should start by copying the relevant files from
`gemyago/golang-backend-boilerplate` at commit
`798f0dc9fd753481d0d698d8232ea08df44185b6`, then minimally adapt them for
Signal Foundry.

## Prerequisite

Production workloads must not depend on persistent local files or volumes.
Complete [Database-Backed State Plan](./database-backed-state-plan.md) before
treating this deployment as production-ready.

The release and Helm work may be developed in parallel, but production rollout
must wait until that prerequisite is complete.

## Release Artifact

Signal Foundry has one release binary and one runtime image.

The frontend is built once and embedded into the Go binary before Go
cross-compilation. The application config YAML, system prompt template, and
database migration logic are also compiled into the binary.

The only separate application runtime assets are the platform-agent skills
under `.platform-agents/skills`. They must be staged with the release artifacts
and copied into the runtime image.

The host build must produce:

- `linux/amd64/signal-foundry`
- `linux/arm64/signal-foundry`
- a staged copy of `.platform-agents/skills`

Release builds must use `CGO_ENABLED=0` and the `release` build tag.

Ignored `*-user.yaml` config files must not be embedded into release binaries.
Only tracked config resources should be embedded; local user config should be
loaded externally at runtime if that workflow remains supported.

## Upstream Build Files

Copy and adapt these upstream areas:

- `build/Makefile`
- `build/build.cfg`
- `build/docker/Dockerfile`
- `build/.crane-version`
- `build/.gitignore`
- `build/README.md`
- `build/scripts/read-build-config.sh`
- `build/scripts/resolve-docker-tags.sh`
- `build/scripts/tag-remote-images.sh`
- `build/scripts/ghcr.py`
- `build/scripts/tests/test_ghcr.py`

Adapt upstream command discovery to one binary named `signal-foundry`. Do not
restore the removed npm package distribution pipeline.

Delete the remaining tracked `build/npm/.gitignore`; ignored local npm staging
outputs are obsolete and are not release inputs.

## Docker Image

Publish one image:

`ghcr.io/gemyago/signal-foundry`

Use a distroless static non-root runtime image. Docker must not compile the UI
or Go code. It only copies the prepared binary for `TARGETOS/TARGETARCH` and the
staged platform-agent skills.

The Dockerfile must:

- select the runtime base for the target platform, not `BUILDPLATFORM`
- copy the binary to a stable path on `PATH`
- copy platform-agent skills to the path expected by application config
- run as a non-root user
- set only `ENTRYPOINT ["signal-foundry"]`
- define no default `CMD`

Running the image without arguments should therefore display Cobra help.
Kubernetes workload arguments select the process mode.

## Pull Request CI

Pull requests targeting `main` should:

- build the complete host release artifacts
- run affected lint and tests
- validate build scripts and Helm rendering
- never publish an image

Do not upload and download build artifacts between jobs. Go cross-compilation is
faster than that transfer. PR builds only verify that the host release build
succeeds; Docker publication is not part of pull request CI.

## Automatic Publication

Pushes to `main` automatically build and publish the image.

Use one build-and-publish job with `contents: read` and `packages: write`. Build
the frontend and both Go binaries directly on the runner, then invoke Docker in
the same workspace to package and push those prepared outputs. Docker still
must not compile application source.

## Manual Publication

Any branch in the repository may be published manually by a maintainer with
write access.

The `workflow_dispatch` workflow itself must run from `main` and accept a
`source_ref` input. The job checks out that source ref, builds on the host, and
packages and publishes from the same runner workspace. Selecting a source branch
is an explicit maintainer trust decision because that branch's build code runs
in a job with package-write permission.

Example:

```sh
gh workflow run publish-docker-image.yml \
  --ref main \
  -f source_ref=feat/example
```

The workflow must reject a manual run whose workflow ref is not the default
branch.

## Image Tags

Set `stable_branches = main`.

Main publications receive:

- `latest-main`
- `git-commit-<sha7>`

Manual branch publications receive:

- `<sanitized-branch>`
- `git-commit-<sha7>`

The immutable commit tag must be present for every published image so releases
can be promoted without rebuilding.

Published stable SemVer releases receive:

- `git-tag-v1.2.3`
- `v1.2.3`
- `v1.2-latest`
- `v1-latest`
- `latest`

Prereleases receive only their full prerelease and `git-tag-*` tags. Release
tagging should use Crane to retag the existing commit image remotely.

Copy and adapt upstream's draft release preparation and published release image
tagging workflows. Do not copy PR-image promotion because Signal Foundry
publishes directly after a push to `main`.

## GHCR Cleanup

Copy upstream's tested GHCR cleanup utility and pinned Python dependencies.
Adapt it for:

- namespace `users/gemyago`
- package `signal-foundry`
- seven-day retention for branch and commit-only images
- permanent retention for stable and release tags

Run cleanup on a schedule and through `workflow_dispatch`. Cleanup must preserve
multi-platform manifests associated with retained tags.

## Helm Chart

Copy the relevant upstream chart scaffold into
`deploy/helm/signal-foundry`, including naming helpers, Service, HTTPRoute,
ServiceAccount, chart test, pinned Helm tooling, and deployment documentation.

Copy the upstream HPA pattern for the app Deployment only, disabled by default.
Database-backed persistence allows app HA. Active agent runs and SSE fan-out
remain process-local, so a reconnect may miss the live run or race with a run on
another replica; this risk is accepted while that surface is not used in
production. Do not offer an HPA for the worker because its cross-replica claim
and recovery behavior can duplicate durable work.

The chart must use the same image for all workloads.

### App Deployment

Name the HTTP workload `app`. It serves the API and embedded frontend.

Its arguments are:

```text
--env production --json-logs start
```

Expose port `4501` through a ClusterIP Service. Use `/health` for startup,
liveness, and initial readiness probes. Document that `/health` currently
checks process health only, not database readiness.

Keep the app replica count configurable and allow normal rolling updates. Do not
force a singleton because of the accepted active-run and SSE reconnect risk.

### Worker Deployment

Name the durable job workload `worker`.

Its arguments are:

```text
--env production --json-logs jobs worker
```

Keep one replica and use a non-overlapping deployment strategy. Worker
horizontal scaling remains blocked until job leases, claim ownership, stale
recovery, and duplicate execution behavior are safe for multiple workers.

The backend CLI must handle `SIGTERM` through context cancellation before this
workload is considered production-ready.

### Scheduler CronJob

Create a Kubernetes CronJob that runs:

```text
--env production --json-logs jobs enqueue-due
```

The schedule is configurable and defaults to once per minute. This is safe for
PostgreSQL: the scheduler performs one small query filtered by enabled and due
state, with `next_run_at` indexed, and writes only when schedules are due. One
query per minute is negligible; the practical tradeoff is up to one minute of
scheduling delay. Add batching or a more selective index only if real schedule
volume justifies it.

Set `concurrencyPolicy: Forbid`, bounded history, a deadline, and
`restartPolicy: OnFailure`.

The CronJob only enqueues due durable jobs. It must not execute long-running
product work.

### Migration Job

Create a Job that runs:

```text
--env production --json-logs db-migrate
```

Annotate it as a Helm `pre-install,pre-upgrade` hook with a negative weight and
`before-hook-creation,hook-succeeded` deletion policy. Argo CD maps these Helm
hooks to `PreSync`, waits for successful completion, and blocks workload rollout
when migration fails.

Keep failed Jobs available for diagnosis. Migrations should remain additive
while old pods may still be running; destructive changes require a deliberate
maintenance deployment.

### Configuration And Secrets

The chart must accept existing ConfigMap and Secret names and load their
`APP_*` entries with `envFrom`. It must never render secret values from chart
values.

Production configuration must provide at least:

- PostgreSQL data-layer DSN
- PostgreSQL agent-runtime DSN
- non-colliding table prefixes
- JWT signing key
- provider credentials used by enabled integrations
- externally correct callback URLs

SQLite remains local-development-only.

The chart must support private GHCR `imagePullSecrets` and image digest pinning.

### Routing

Include the upstream Gateway API `HTTPRoute` pattern, disabled by default.
Gateway name, namespace, section, hostnames, matching rules, and filters must be
values-driven. Do not add an Ingress in the initial chart.

### Security

Default to:

- non-root execution
- disabled service-account token automounting
- no privilege escalation
- all Linux capabilities dropped
- `RuntimeDefault` seccomp
- JSON logs to stdout

Persistent volumes are not part of the production chart. Optional file-based
integrations such as application TLS or provider private-key files must be
handled separately or changed to consume secret values before being enabled.

## Argo CD Boundary

This repository contains the Helm chart but no live Argo CD `Application`.
Cluster URL, namespace, gateway, secret management, and environment-specific
values belong to the deployment/config repository.

Argo CD should deploy an immutable `git-commit-*` tag or image digest. Publishing
an image does not by itself update Git desired state. The deployment repository
or Argo CD Image Updater must promote the selected digest.

## Validation

Add Nx projects or equivalent affected-task wiring for build scripts and Helm.

Required verification:

1. Run build-script unit and tag-resolution tests.
2. Run GHCR cleanup unit tests.
3. Build Linux `amd64` and `arm64` artifacts on the host.
4. Verify the build output contains only expected binaries and runtime assets.
5. Build the multi-platform Docker image.
6. Verify the image without arguments displays help.
7. Verify each workload command resolves successfully.
8. Run `helm lint` and render default and production-like values.
9. Verify the migration hook, two Deployments, CronJob, Service, and optional HTTPRoute.
10. Run `make affected-lint-test`.
11. Run a PostgreSQL smoke flow covering migration, API health, worker startup, and one scheduler tick.

## Documentation Updates

Update these documents with the implementation:

- root `AGENTS.md`
- `build/AGENTS.md`
- `apps/signal-foundry/AGENTS.md`
- new `deploy/AGENTS.md`
- `docs/ARCHITECTURE.md`
- build and deployment READMEs
