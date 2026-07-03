## ADDED Requirements

### Requirement: Finance Tenants Can Be Archived
The finance module SHALL let authenticated tenant members archive finance tenants through the backend API without hard-deleting tenant-owned finance history.

#### Scenario: Tenant member archives a finance tenant
- **WHEN** an authenticated tenant member calls the tenant archive API for a joined finance tenant
- **THEN** the system MUST persist archive state on that tenant instead of deleting the tenant and its tenant-owned records
- **AND** the first implementation MUST allow any current tenant member to perform the archive because finance tenant members are otherwise equal
- **AND** the archive mutation MUST remain on the existing camelCase `/api/v1/finance/...` API surface
- **AND** the archive mutation MUST NOT return tenant data unless the backend generates response data the client needs immediately

#### Scenario: Archived tenant leaves active workspace flows
- **WHEN** a finance tenant has been archived
- **THEN** it MUST stop appearing in the default joined-tenant list returned for that member
- **AND** `GET /api/v1/finance/tenants/{tenantId}` MUST no longer return it as an active tenant selection result

#### Scenario: Tenant archive remains API-only and one-way for this slice
- **WHEN** finance tenant archive support is delivered in this change
- **THEN** the backend MUST expose the archive behavior without requiring any new UI route or UI interaction
- **AND** the system MUST NOT introduce tenant restore/unarchive or hard-delete behavior as part of this slice
