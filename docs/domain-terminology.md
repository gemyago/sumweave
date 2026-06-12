# Domain Terminology

Canonical product vocabulary for planning, design, and copy.

This glossary follows [ARCHITECTURE.md](./ARCHITECTURE.md). If another document conflicts with that architecture, the architecture wins.

## Glossary

| Term | Definition |
| --- | --- |
| `runtime` | The core Go module where shared product logic and slice boundaries should live. |
| `shared domain` | Small, stable product concepts shared across slices, such as identifiers, market data records, analytics outputs, strategy records, and execution records. |
| `slice` | A focused runtime area with a clear responsibility. The intended main slices are `data`, `analytics`, `strategy`, `governor`, and `execution`. |
| `data` | The slice responsible for canonical market and reference data, normalization, quality state, and replayable persistence. |
| `analytics` | Deterministic analysis built on top of data. |
| `strategy` | Deterministic strategy logic that turns data and analytics into candidate actions. |
| `governor` | The risk and policy gate between strategy output and live execution. |
| `execution` | The slice that owns order, fill, reconciliation, and venue-facing behavior after approval. |
| `operator UI` | The human-facing application in `apps/signal-ui/`. |
| `venue integration` | Exchange or broker specific code isolated at the system edge behind narrow product-facing interfaces. |
| `deterministic path` | The critical flow `Data -> Analytics -> Strategy -> Governor -> Execution`. AI must stay outside this path. |
| `AI-assisted research` | Non-critical support work such as research, drafting, critique, explanation, and summarization. |

## Usage Notes

- Prefer these terms in docs, code comments, and UI copy.
- Avoid reviving agent-control-plane language unless a future architecture document explicitly reintroduces it.
- When a lower-level implementation uses older names, treat those as inherited technical details rather than product terminology.
