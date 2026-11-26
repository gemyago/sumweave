---
type: Guide
title: "Maintaining the knowledge bundle"
description: "OKF 0.3 metadata, navigation, lifecycle, and OpenSpec responsibilities."
status: stable
sources:
  - resource: "https://github.com/open-knowledge-format/open-knowledge-format/blob/527e81aff55992be21507e97004286789126f88c/SPEC.md"
---

# Maintaining the knowledge bundle

`docs/` is one Open Knowledge Format (OKF) 0.3 bundle, entered through
[index.md](../index.md). The maintained [official specification](https://github.com/open-knowledge-format/open-knowledge-format/blob/527e81aff55992be21507e97004286789126f88c/SPEC.md)
is the format reference. The root index declares `okf_version: "0.3"`, title,
and description; it is an index, not a concept. Subdirectory `index.md` files
contain navigation with no frontmatter. README files are human entry redirects
or runbooks and are deliberately skipped unless they carry `type`.

Intended concepts start with YAML 1.2 mapping frontmatter between exact `---`
lines. `type` is required and nonempty. Use a useful title and one-sentence
description; keep unknown existing metadata when editing. Current references use
`status: stable`, unfinished proposals use `draft`, and retained superseded
sources use `deprecated` with bundle-root `superseded_by` where applicable.
Experimental is a product-scope label, not a lifecycle status. Stable means ready
to consume; it does not claim human review or a completed E2E run.

Record meaningful provenance in `sources`, each with a `resource`. Bundle paths
are appropriate for in-bundle sources. For repository evidence outside `docs/`,
use an absolute repository URL or a clearly worded scope descriptor, then provide
ordinary repository links in the body. Do not invent verification events, human
attribution, generation dates, or freshness deadlines. Per-claim source attribution
can use a footnote label matching a source `id`.

## Paths and navigation

Standard Markdown links resolve relative to their containing document. Leading
slashes mean the **bundle root**, not the repository root. Encode spaces and
reserved URI characters. `superseded_by: /product/overview.md`, for example,
resolves to `docs/product/overview.md`.

Links such as `../../openspec/specs/finance-management/spec.md` work in a repository
browser but are external to this bundle; OKF consumers must not read outside the
bundle from the local filesystem. Such references are labeled as repository
sources. Keep OpenSpec, source code, build/deploy runbooks, UI design references,
and test instructions in their owning directories; do not copy them merely to
make an internal graph edge.

Add useful concepts to the nearest index and link them from their reader's
journey. Before moving a page, search normal and hidden references and retain a
deprecated redirect at the old path. Preserve meaningful contracts, schema,
limits, commands, examples, troubleshooting, and accepted risks. Historical
records can retain old wording under a visible warning; do not silently promote
planned functionality to current behavior.

## OpenSpec and current knowledge

[Architecture](../ARCHITECTURE.md) remains the product-direction authority.
Topic references describe current implemented behavior and limitations.
[OpenSpec capability specifications](../../openspec/specs/) retain detailed
requirements and scenarios; [active changes](../../openspec/changes/) retain
proposals, designs, tasks, and deltas. Completed changes stay in the
[archive](../../openspec/changes/archive/). These repository paths are outside
this bundle.

Keep the [contributor workflow](../../CONTRIBUTING.md#openspec-workflow), existing
manager prompts, `openspec propose`/`openspec apply`, artifact locations, and
natural-language approval. OKF complements that workflow. When a legacy scenario
and implementation differ, identify the exact discrepancy in the topic reference
and resolve it deliberately through OpenSpec rather than rewriting history or
assuming every old plan is implemented.

Current [classification](../product/classification.md) scenarios retain the
accepted shared safe error contract and identify the two early create/replace
empty-`400` implementation gaps. [Matching](../product/transfer-matching.md)
scenarios cover all four delivered rules. Keep these current capability files
consistent with accepted contracts and implementation while preserving archived
change records.

For documentation changes, check YAML syntax/duplicate keys, lifecycle values,
index semantics, provenance/path scope, links and anchors, and reader journeys.
For content changes compare formal scenarios and representative implementation;
valid metadata alone does not establish truth.
