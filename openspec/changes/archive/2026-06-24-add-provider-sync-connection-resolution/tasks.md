## 1. Connector Registry

- [x] 1.1 Add the provider-sync v2 `ConnectorRegistry` contract and a finance-owned static registry in `finance/internal/providers/`, and must follow TDD flow by first writing failing tests proving the registry resolves connectors by declared `ConnectorID`, supports the current `monobank` and `enable-banking` technical connectors, and returns bounded errors for empty or unknown connector IDs before implementing and verifying focused tests.
- [x] 1.2 Wire the current provider-sync v2 connector construction through the registry-backed coordinator setup, and must follow TDD flow by first writing failing wiring tests proving PKO continues to compose through `enable-banking`, monobank keeps its direct connector registration, and supported connectors are registered exactly once before implementing and verifying focused tests.

## 2. Coordinator Resolution

- [x] 2.1 Update `SyncCoordinator` to depend on the registry and resolve fetch connectors from `request.Connection.ConnectorID`, and must follow TDD flow by first writing failing coordinator tests proving monobank connections resolve `monobank`, PKO connections with `ConnectorID: enable-banking` resolve the Enable Banking connector, and the coordinator uses the resolved connector instead of product-provider-specific branching before implementing and verifying focused tests.
- [x] 2.2 Add early connector-resolution failure handling in `SyncCoordinator`, and must follow TDD flow by first writing failing coordinator tests proving empty or unconfigured connector IDs fail before any fetch call, return a bounded connector-resolution error, and do not expose connection secrets or raw payload content before implementing and verifying focused tests.
