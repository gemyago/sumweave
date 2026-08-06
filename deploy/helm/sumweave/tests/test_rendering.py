#!/usr/bin/env python3
"""Rendering contract tests for native Kubernetes environment values."""

import subprocess
import sys
import tempfile
from pathlib import Path


HELM, CHART = sys.argv[1:3]
PRODUCTION_VALUES = Path(CHART) / "values-production.example.yaml"


def render(values=None):
    command = [HELM, "template", "sumweave", CHART]
    if values is not None:
        command.extend(["-f", str(values)])
    result = subprocess.run(command, check=False, capture_output=True, text=True)
    if result.returncode:
        raise AssertionError(f"helm template failed:\n{result.stderr}")
    return result.stdout


def document(rendered, kind, name_suffix):
    for item in rendered.split("\n---\n"):
        if f"kind: {kind}\n" in item and f"name: sumweave-sumweave-{name_suffix}\n" in item:
            return item
    raise AssertionError(f"missing {kind} named *-{name_suffix}")


def container(rendered, kind, workload, name):
    manifest = document(rendered, kind, workload)
    marker = f"- name: {name}\n"
    start = manifest.find(marker)
    if start == -1:
        raise AssertionError(f"missing {name} container in {workload}")
    return manifest[start:]


def require(text, value):
    if value not in text:
        raise AssertionError(f"expected {value!r}")


def forbid(text, value):
    if value in text:
        raise AssertionError(f"did not expect {value!r}")


def render_with(content):
    with tempfile.NamedTemporaryFile("w", suffix=".yaml") as values:
        values.write(content)
        values.flush()
        return render(values.name)


def render_failure(content):
    with tempfile.NamedTemporaryFile("w", suffix=".yaml") as values:
        values.write(content)
        values.flush()
        result = subprocess.run(
            [HELM, "template", "sumweave", CHART, "-f", values.name],
            check=False,
            capture_output=True,
            text=True,
        )
    if result.returncode == 0:
        raise AssertionError("expected helm template to fail")
    return result.stderr


def runtime_containers(rendered):
    return (
        container(rendered, "Deployment", "app", "app"),
        container(rendered, "Deployment", "worker", "worker"),
        container(rendered, "CronJob", "scheduler", "scheduler"),
    )


def test_omitted_defaults():
    rendered = render()
    for item in (*runtime_containers(rendered), container(rendered, "Job", "migrate", "migrate")):
        forbid(item, "env:\n")
        forbid(item, "envFrom:\n")
    forbid(rendered, "name: sumweave-sumweave-initial-user\n")
    forbid(rendered, "kind: Secret\n")
    forbid(rendered, "stringData:\n")


def test_migration_uses_default_service_account_without_token():
    rendered = render()
    migration = document(rendered, "Job", "migrate")

    require(migration, "automountServiceAccountToken: false")
    forbid(migration, "serviceAccountName:")
    require(migration, 'helm.sh/hook-weight: "-10"')


def test_initial_user_uses_external_secret_after_migration():
    rendered = render_with(
        """\
initialUser:
  enabled: true
  secret:
    name: initial-user-secret
    usernameKey: login
    passwordKey: initial-password
  env:
    - name: INITIAL_USER_DB_HOST
      value: database.example
    - name: APP_APPLICATION_DATABASE_DSN
      value: postgres://app@$(INITIAL_USER_DB_HOST)/sumweave
  envFrom:
    - secretRef:
        name: initial-user-database-secret
"""
    )
    job = document(rendered, "Job", "initial-user")
    initial_user = container(rendered, "Job", "initial-user", "initial-user")

    require(job, "automountServiceAccountToken: false")
    forbid(job, "serviceAccountName:")
    require(job, "helm.sh/hook: pre-install")
    forbid(job, "helm.sh/hook: pre-install,pre-upgrade")
    require(job, 'helm.sh/hook-weight: "-5"')
    require(initial_user, "--if-not-exists")
    require(initial_user, "$(INITIAL_USER_USERNAME)")
    require(initial_user, "$(INITIAL_USER_PASSWORD)")
    require(initial_user, "name: INITIAL_USER_USERNAME")
    require(initial_user, "name: INITIAL_USER_PASSWORD")
    require(initial_user, "name: initial-user-secret")
    require(initial_user, "key: login")
    require(initial_user, "key: initial-password")
    require(initial_user, "name: INITIAL_USER_DB_HOST")
    require(initial_user, "name: APP_APPLICATION_DATABASE_DSN")
    require(initial_user, "name: initial-user-database-secret")
    if initial_user.index("name: INITIAL_USER_DB_HOST") > initial_user.index(
        "name: APP_APPLICATION_DATABASE_DSN"
    ):
        raise AssertionError("initial user env order was not preserved")
    for item in (*runtime_containers(rendered), container(rendered, "Job", "migrate", "migrate")):
        forbid(item, "initial-user-secret")
        forbid(item, "INITIAL_USER_DB_HOST")
        forbid(item, "initial-user-database-secret")
    forbid(rendered, "kind: Secret\n")
    forbid(rendered, "stringData:\n")


def test_initial_user_requires_external_secret_name():
    error = render_failure("initialUser:\n  enabled: true\n")
    require(error, "initialUser.secret.name is required when initialUser.enabled is true")


def test_scope_propagation_and_native_fields():
    rendered = render_with(
        """\
env:
  - name: RUNTIME_ORDER_FIRST
    value: first
  - name: RUNTIME_ORDER_SECOND
    value: $(RUNTIME_ORDER_FIRST)
envFrom:
  - prefix: RUNTIME_CONFIG_
    configMapRef:
      name: runtime-config
      optional: true
  - prefix: RUNTIME_SECRET_
    secretRef:
      name: runtime-secret
      optional: false
migration:
  env:
    - name: MIGRATION_ORDER_FIRST
      value: first
    - name: MIGRATION_ORDER_SECOND
      value: $(MIGRATION_ORDER_FIRST)
  envFrom:
    - prefix: MIGRATION_CONFIG_
      configMapRef:
        name: migration-config
        optional: true
"""
    )
    for item in runtime_containers(rendered):
        require(item, "name: RUNTIME_ORDER_FIRST")
        require(item, "name: RUNTIME_ORDER_SECOND")
        require(item, "value: $(RUNTIME_ORDER_FIRST)")
        require(item, "name: runtime-config")
        require(item, "name: runtime-secret")
        require(item, "prefix: RUNTIME_CONFIG_")
        require(item, "prefix: RUNTIME_SECRET_")
        require(item, "optional: true")
        require(item, "optional: false")
        if item.index("name: RUNTIME_ORDER_FIRST") > item.index("name: RUNTIME_ORDER_SECOND"):
            raise AssertionError("runtime env order was not preserved")
        forbid(item, "MIGRATION_ORDER_FIRST")
        forbid(item, "migration-config")

    migrate = container(rendered, "Job", "migrate", "migrate")
    require(migrate, "name: MIGRATION_ORDER_FIRST")
    require(migrate, "name: MIGRATION_ORDER_SECOND")
    require(migrate, "value: $(MIGRATION_ORDER_FIRST)")
    require(migrate, "name: migration-config")
    require(migrate, "prefix: MIGRATION_CONFIG_")
    require(migrate, "optional: true")
    if migrate.index("name: MIGRATION_ORDER_FIRST") > migrate.index("name: MIGRATION_ORDER_SECOND"):
        raise AssertionError("migration env order was not preserved")
    forbid(migrate, "RUNTIME_ORDER_FIRST")
    forbid(migrate, "runtime-config")


def test_scopes_do_not_inherit_when_only_one_is_set():
    runtime_only = render_with(
        """\
env:
  - name: RUNTIME_ONLY_CREDENTIAL
    value: runtime-only
envFrom:
  - secretRef:
      name: runtime-only-secret
"""
    )
    for item in runtime_containers(runtime_only):
        require(item, "RUNTIME_ONLY_CREDENTIAL")
        require(item, "runtime-only-secret")
    migrate = container(runtime_only, "Job", "migrate", "migrate")
    forbid(migrate, "RUNTIME_ONLY_CREDENTIAL")
    forbid(migrate, "runtime-only-secret")
    forbid(migrate, "env:\n")
    forbid(migrate, "envFrom:\n")

    migration_only = render_with(
        """\
migration:
  env:
    - name: MIGRATION_ONLY_CREDENTIAL
      value: migration-only
  envFrom:
    - secretRef:
        name: migration-only-secret
"""
    )
    for item in runtime_containers(migration_only):
        forbid(item, "MIGRATION_ONLY_CREDENTIAL")
        forbid(item, "migration-only-secret")
        forbid(item, "env:\n")
        forbid(item, "envFrom:\n")
    migrate = container(migration_only, "Job", "migrate", "migrate")
    require(migrate, "MIGRATION_ONLY_CREDENTIAL")
    require(migrate, "migration-only-secret")


def test_production_example():
    rendered = render(PRODUCTION_VALUES)
    for item in runtime_containers(rendered):
        require(item, "name: sumweave-db-secret")
        require(item, "name: APP_FINANCE_PROVIDERS_ENABLEBANKING_PRIVATEKEYPATH")
        require(item, "value: /var/run/secrets/sumweave/enable-banking/private-key.pem")
        require(item, "mountPath: /var/run/secrets/sumweave/enable-banking")
        require(item, "secretName: sumweave-app-secrets")
        require(item, "key: enable-banking-private-key.pem")
        require(item, "path: private-key.pem")
        require(item, "value: postgres://$(DB_USERNAME):$(DB_PASSWORD)@$(DB_HOST):5432/sumweave?sslmode=require")
        forbid(item, "name: sumweave-ops-db-secret")
    migrate = container(rendered, "Job", "migrate", "migrate")
    require(migrate, "name: sumweave-ops-db-secret")
    require(migrate, "value: postgres://$(DB_USERNAME):$(DB_PASSWORD)@$(DB_HOST):5432/sumweave?sslmode=require")
    forbid(migrate, "name: sumweave-db-secret")
    forbid(migrate, "enable-banking-private-key")
    initial_user = container(rendered, "Job", "initial-user", "initial-user")
    require(initial_user, "name: sumweave-initial-user")
    require(initial_user, "name: sumweave-db-secret")
    require(initial_user, "value: postgres://$(DB_USERNAME):$(DB_PASSWORD)@$(DB_HOST):5432/sumweave?sslmode=require")
    forbid(initial_user, "name: sumweave-ops-db-secret")
    forbid(initial_user, "enable-banking-private-key")
    forbid(rendered, "kind: Secret\n")
    forbid(rendered, "stringData:\n")


def main():
    test_omitted_defaults()
    test_migration_uses_default_service_account_without_token()
    test_initial_user_uses_external_secret_after_migration()
    test_initial_user_requires_external_secret_name()
    test_scope_propagation_and_native_fields()
    test_scopes_do_not_inherit_when_only_one_is_set()
    test_production_example()


if __name__ == "__main__":
    main()
