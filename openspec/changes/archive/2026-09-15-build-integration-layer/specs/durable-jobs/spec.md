## MODIFIED Requirements

### Requirement: Metadata-only jobs API

The backend application SHALL expose authenticated, caller-scoped
`GET /api/v1/jobs` and `GET /api/v1/jobs/{jobId}` endpoints using camelCase JSON.

#### Scenario: Session caller lists own user-requested jobs

- **WHEN** an authenticated browser-session caller requests the jobs list with
  supported metadata filters
- **THEN** the response MUST return only rows whose `requester_user_id` equals
  the caller and whose source is `operator` or `integration`
- **AND** a requested source filter MUST be intersected with those allowed
  sources
- **AND** the deterministic page MUST include safe lifecycle, requester,
  attempt, and terminal-error information
- **AND** it MUST NOT expose arbitrary input, progress, result, dispatch, or
  dead-letter payloads.

#### Scenario: Access token lists own integration jobs

- **WHEN** an authenticated access-token caller requests the jobs list
- **THEN** the response MUST return only rows whose `requester_user_id` equals
  the token owner and whose source is `integration`
- **AND** source filters MUST NOT broaden that scope.

#### Scenario: Caller reads an accessible job

- **WHEN** an authenticated caller requests job detail
- **THEN** the storage query MUST constrain job ID, requester user ID, and
  allowed requester sources
- **AND** the response MUST return safe lifecycle, requester, attempt, and
  terminal-error information without `workerId` or arbitrary payloads.

#### Scenario: Missing and inaccessible jobs are indistinguishable

- **WHEN** a caller requests an unknown job, another user's job, or a job from a
  requester source not allowed for that credential
- **THEN** the API MUST return the same `404` response.

#### Scenario: Jobs API requires authentication

- **WHEN** a caller without a valid authenticated identity calls either jobs
  endpoint
- **THEN** the system MUST reject the request as unauthorized.

## ADDED Requirements

### Requirement: Integration Requests Produce Ordinary Observed Jobs

Jobs SHALL accept `integration` as requester metadata without changing the
appdispatch execution or lazy observation model.

#### Scenario: Integration command is first delivered

- **WHEN** an observed finance command with requester source `integration` is
  first delivered
- **THEN** the consumer MUST lazily materialize a job with the dispatch message
  ID and integration requester metadata before execution
- **AND** the existing queued, running, succeeded, failed, retry, recovery, and
  terminal redelivery behavior MUST remain unchanged.

#### Scenario: Revocation does not cancel accepted work

- **WHEN** an access token is revoked after its command was durably accepted
- **THEN** the worker MUST continue normal delivery and execution
- **AND** the revoked token MUST be unable to read the resulting job on later
  requests.
