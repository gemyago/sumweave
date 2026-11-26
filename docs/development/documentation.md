---
type: Guide
title: "Maintaining the knowledge bundle"
description: "OKF 0.3 authoring, sources, trust, lifecycle, navigation, and validation."
sources:
  - id: okf-spec
    resource: "https://github.com/open-knowledge-format/open-knowledge-format/blob/527e81aff55992be21507e97004286789126f88c/SPEC.md"
    title: "Official OKF 0.3 specification"
  - id: okf-generate
    resource: "https://github.com/open-knowledge-format/open-knowledge-format/blob/527e81aff55992be21507e97004286789126f88c/skills/okf-generate/SKILL.md"
    title: "Official OKF generation skill"
  - id: repository-policy
    resource: "repository documentation policy in AGENTS.md and CONTRIBUTING.md"
    title: "Sumweave documentation policy"
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
---

# Maintaining the knowledge bundle

`docs/` is one Open Knowledge Format (OKF) 0.3 bundle, entered through
[index.md](../index.md). Product and architecture documentation belongs here.
[Architecture](../ARCHITECTURE.md) remains the product-direction authority;
[requirements](../requirements/index.md) retain detailed acceptance scenarios.
The current topic references describe delivered behavior and limitations.[^repository-policy]

Use the local [okf-generate skill](../../.agents/skills/okf-generate/SKILL.md) for
writing, migration, repair, and validation, and
[okf-consume](../../.agents/skills/okf-consume/SKILL.md) for answers from the bundle.
The [official skills source](https://github.com/open-knowledge-format/open-knowledge-format/tree/main/skills)
and [local provenance](../../.agents/skills/OKF-UPSTREAM.md) identify the maintained
upstream, exact copied revision, resources, and Apache-2.0 license. These repository
links are outside the bundle; an OKF consumer must not traverse them from its local
filesystem. A repository contributor can open them directly.[^repository-policy]

## Concepts and sources

A concept starts with YAML 1.2 mapping frontmatter between exact `---` lines.
`type` is required and nonempty. Use the existing descriptive types, a useful
title, and a one-sentence description. Preserve unknown metadata, human-authored
content, and verification history. Use stable lowercase names for new concepts;
keep established paths such as `ARCHITECTURE.md` to preserve incoming links.[^okf-spec]

Give every source a stable `id`, a `resource`, and a `title`. Prefer bundle-root
paths such as `/requirements/transaction-classification.md` for in-bundle sources.
For repository evidence outside `docs/`, use a pinned repository URL or an explicit
scope descriptor. Attribute specific claims with `[^source-id]` footnotes matching
the source IDs. A source snapshot establishes provenance, not proof of delivery.
Use implementation and tests to check a current-behavior claim; label a contract
as required when implementation differs.[^okf-generate]

## Authorship, trust, and lifecycle

Set `generated.by` to the actual tool/model actor on new concepts and meaningful
content changes, with the real `generated.at` timestamp including its offset.
Preserve existing generation metadata for format, whitespace, link-only repairs,
and moves. Do not infer the producer or time of an old source body: a migration
wrapper identifies its own authorship and cites the historical original.[^okf-generate]

Do not add `verified` for content generated in the same run. Record a human
verification only when that person explicitly confirmed the content, asked for it
to be recorded, and supplied their identity. Without such an event a concept is
`unverified`, even if it is stable and its structural checks pass. Only events at
or after `generated.at` give a current trust tier. Machine-confirmed and
human-reviewed definitions remain separate from runtime test execution.[^okf-generate]

Omit `status` for normal consumable content (the default is `stable`). Use `draft`
for incomplete content needing owner review. Mark retained retired sources
`deprecated`, with bundle-root `superseded_by` pointing to an actual replacement
when one exists, and explain why in the body. Experimental is a product-scope
label, not a lifecycle value. Set `stale_after` only for a known refresh cycle;
absence means no freshness deadline is asserted. Do not invent owners, review
cycles, verification events, sanctioned computations, or executed tests.[^okf-spec]

## Navigation and history

Write body links relative to their containing document. Leading `/` in links or
path fields means the bundle root, not the repository root. Encode spaces and
reserved URI characters. Paths outside the bundle are external and cannot be
read through the consumer's local filesystem. Keep OpenAPI, implementation,
module runbooks, and harness instructions at their owning repository locations;
label external references clearly.[^okf-spec]

Update `index.md` in each changed directory. Group entries under headings, copy
concept descriptions, and use `* [Title](path) - description`. Only the root index
has frontmatter; preserve its existing bundle metadata. Add changes to
[log.md](../log.md), with no frontmatter, newest `## YYYY-MM-DD` first.[^okf-generate]

Retain granular requirements and scenarios in `docs/requirements/`, linked both
from and to the relevant topic. Keep valuable retired proposals, deltas, task
records, designs, and reviews in [history](../history/index.md). Their old commands,
plans, and test claims describe past work only. Tooling metadata and workflow
prompts are removed. Archived source snapshots remain available through provenance
URLs; their recorded dates and original authorship are not regenerated.[^repository-policy]

Normally keep a deprecated redirect when moving a concept. The explicit removal
of the former tooling tree requires removing its old paths entirely; update
repository backlinks and use the pinned original URLs for historical provenance
instead. For remaining retired concepts already in this bundle, keep their bodies
and redirect metadata at the established paths.[^repository-policy]

## Contracts and known gaps

[Classification](../product/classification.md) and its
[requirements](../requirements/transaction-classification.md) preserve the
accepted safe JSON error contract, while documenting the two existing early
create/replace empty-`400` paths. [Matching](../product/transfer-matching.md) and its
[requirements](../requirements/transfer-matching.md) describe all four delivered
rules. Reconcile current contracts and delivery explicitly; old archived deltas
remain historical rather than redefining current behavior.[^repository-policy]

## Validation

From the repository root, run the copied upstream tools (Python 3.9+ and PyYAML):

```sh
python3 .agents/skills/okf-generate/scripts/validate.py docs
python3 .agents/skills/okf-consume/scripts/scan.py docs --json
```

Fix every validator error. Fix warnings or disclose why they are kept. `W120` is
expected only for deliberate non-concepts such as README redirects. Also check
YAML 1.2 and duplicate keys, source IDs and footnotes, lifecycle and trust fields,
path scope, case-sensitive targets, anchors, index reachability, source/content
preservation, and representative reader journeys. The validator does not prove
that a requirement shipped or that a runbook was executed.[^okf-generate]

For code/config changes, follow the repository coding completion protocol,
including PostgreSQL bootstrap before backend tests and `make affected-lint-test`.
Markdown-only work uses the non-coding protocol. Report checks actually performed,
remaining uncertainty, and implementation gaps separately from formatting.[^repository-policy]

[^okf-spec]: Official OKF 0.3 specification, especially sections 3-9 and 11.
[^okf-generate]: Official OKF generation skill, including provenance and validation.
[^repository-policy]: Root repository instructions and contributor documentation workflow.
