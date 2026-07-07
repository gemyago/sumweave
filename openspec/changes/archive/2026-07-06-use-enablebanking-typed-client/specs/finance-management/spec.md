## ADDED Requirements

### Requirement: Enable Banking Connector Uses Typed Client
The finance module SHALL implement the provider sync v2 Enable Banking connector through the generated typed Enable Banking client surface rather than connector-owned raw HTTP request and response mapping.

#### Scenario: Redirect link start uses typed auth creation
- **WHEN** the Enable Banking connector starts a PKO redirect/SCA link through the supported official branch
- **THEN** it MUST call the generated typed auth-creation client operation with a typed request model
- **AND** it MUST build the provider sync v2 start-link result from the typed auth response
- **AND** it MUST NOT read raw response maps or call raw transport helpers to derive start-link fields

#### Scenario: Redirect link finish uses typed session creation
- **WHEN** the Enable Banking connector finishes a PKO redirect/SCA link with a provider code
- **THEN** it MUST call the generated typed session-creation client operation with a typed request model
- **AND** it MUST build the provider sync v2 link result from the typed session response
- **AND** it MUST NOT read raw response maps or call raw transport helpers to derive finish-link fields

#### Scenario: Fetch uses typed session, balance, and transaction operations
- **WHEN** the Enable Banking connector fetches a requested provider sync window for a linked PKO connection
- **THEN** it MUST call generated typed client operations for session/account, balance, and paged transaction data
- **AND** it MUST map typed responses into provider account, balance, transaction, provider-original, and fingerprint observations
- **AND** continuation handling MUST use the typed transaction response continuation key rather than connector-local raw response probing

#### Scenario: Connector raw access is forbidden
- **WHEN** the Enable Banking connector implements normal start, finish, or fetch behavior
- **THEN** connector code MUST NOT call generated raw transport helpers
- **AND** connector code MUST NOT read generated response `Raw` maps or connector-local raw maps for provider field extraction
- **AND** any response data not exposed by the generated typed client MUST be treated as unavailable to the connector rather than recovered through raw fallback behavior
