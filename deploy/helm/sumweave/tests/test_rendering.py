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
    forbid(rendered, "kind: Secret\n")
    forbid(rendered, "stringData:\n")


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
        require(item, "example-sumweave-runtime-config")
        require(item, "example-sumweave-runtime-settings")
        require(item, "example-sumweave-runtime-database-identity")
        require(item, "value: postgres://$(DB_USERNAME):$(DB_PASSWORD)@runtime-postgresql.example.invalid")
        forbid(item, "example-sumweave-migration-config")
        forbid(item, "example-sumweave-migration-settings")
        forbid(item, "example-sumweave-migration-database-identity")
    migrate = container(rendered, "Job", "migrate", "migrate")
    require(migrate, "example-sumweave-migration-config")
    require(migrate, "example-sumweave-migration-settings")
    require(migrate, "example-sumweave-migration-database-identity")
    require(migrate, "value: postgres://$(DB_USERNAME):$(DB_PASSWORD)@migration-postgresql.example.invalid")
    forbid(migrate, "example-sumweave-runtime-config")
    forbid(migrate, "example-sumweave-runtime-settings")
    forbid(migrate, "example-sumweave-runtime-database-identity")
    forbid(rendered, "kind: Secret\n")
    forbid(rendered, "stringData:\n")


def main():
    test_omitted_defaults()
    test_scope_propagation_and_native_fields()
    test_scopes_do_not_inherit_when_only_one_is_set()
    test_production_example()


if __name__ == "__main__":
    main()
