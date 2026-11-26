---
type: Runbook
title: "Release and deployment"
description: "Host-built artifacts, container packaging, and deployment entry points."
sources:
  - id: readme
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/build/README.md"
    title: "build/README.md"
  - id: readme-2
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/deploy/README.md"
    title: "deploy/README.md"
  - id: documentation-snapshot
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/docs/operations/release-deployment.md"
    title: "Reviewed documentation checkpoint before this format migration"
---

# Release and deployment

Build release artifacts on the host with `make -C build dist`. It builds the SPA
once, embeds it into CGO-disabled Linux amd64/arm64 Go binaries, and stages the
platform-agent skill root. Docker only packages those outputs. Embedded UI
serving is used at runtime; an absent embedded `dist/index.html` leaves the
backend API-only. The skill root is intentionally empty until finance-specific
skills are requested; packaging it does not make generic agents product scope.[^documentation-snapshot][^readme][^readme-2]

The following repository runbooks are authoritative and external to this bundle:

- [Release build](../../build/README.md): artifact commands, image publication,
  release tags, cleanup, and the automatic `docker-image-head` pointer.
  Automatic successful main pushes advance that pointer; manual builds do not.
- [Deployment](../../deploy/README.md): app, singleton worker, scheduler CronJob,
  explicit migration hook, optional initial-user hook, environment/secret scopes,
  image pins, gateway routing, and render/test commands.

API, worker, and scheduler share the application database and remain separate
workloads. PostgreSQL is the sole supported database. Runtime workloads use a
DML/query identity; the migration hook uses a separate DDL-capable identity.
Consumers own Secrets/ConfigMaps, signing keys, provider credentials, callback
URLs, and private-registry pull credentials. Migration and initial-user env
scopes do not inherit the app/worker/scheduler environment.

Run `make -C deploy lint` before deployment changes. `/health` currently proves
process health, not database readiness. Optional file TLS/private-key integrations
need explicit secret/volume decisions. See [persistence](../architecture/persistence.md)
for migration ownership and [background processing](../architecture/background-processing.md)
for retries, worker recovery, retention policy, and shutdown ordering.

[^documentation-snapshot]: Reviewed documentation checkpoint, preserved as the source of this section; format and navigation changes do not establish execution results.

[^readme]: build/README.md.
[^readme-2]: deploy/README.md.
