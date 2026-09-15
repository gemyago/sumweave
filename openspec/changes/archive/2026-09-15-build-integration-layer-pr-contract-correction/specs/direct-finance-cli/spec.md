## MODIFIED Requirements

### Requirement: Direct Client Enforces Transport Safety

The direct client SHALL use bounded requests and normal certificate verification
for every remote API call.

#### Scenario: Non-loopback plain HTTP is rejected

- **WHEN** the configured base URL uses plain HTTP outside `localhost`,
  `127.0.0.1`, or `[::1]`
- **THEN** the client MUST reject the configuration before sending credentials.

#### Scenario: HTTPS verification remains enabled

- **WHEN** the client sends an HTTPS request
- **THEN** it MUST use normal Go TLS certificate verification
- **AND** it MUST apply a finite request timeout.

#### Scenario: Redirect destinations are validated before credentials are sent

- **WHEN** a credential-bearing direct-client request receives a redirect
- **THEN** the client MUST validate the proposed destination with the same
  loopback-only plain-HTTP and HTTPS requirements as the configured base URL
- **AND** it MUST reject an unsafe destination, including an HTTPS-to-HTTP
  downgrade, before forwarding the `Authorization` header
- **AND** a redirect policy MUST NOT disable normal Go TLS certificate
  verification or the finite request timeout.

### Requirement: Direct Client Waits For Observed Jobs Safely

The direct client SHALL provide bounded polling for a known job reference while
respecting lazy observed-job materialization.

#### Scenario: Initial not-found is pending briefly

- **WHEN** `swmd job wait` receives `404` for the explicitly supplied expected
  job ID during the first 30 seconds of its overall timeout
- **THEN** it MUST continue polling at the configured finite interval.

#### Scenario: Late not-found is terminal

- **WHEN** the same job remains `404` after the materialization grace period
- **THEN** the client MUST stop with a nonzero not-found result.

#### Scenario: Terminal job ends waiting

- **WHEN** the requested job becomes succeeded or failed
- **THEN** the client MUST write the terminal job JSON
- **AND** it MUST exit zero only for success and nonzero for failure
- **AND** exceeding the overall timeout MUST return nonzero.

#### Scenario: Overall wait deadline bounds an in-flight poll

- **WHEN** `swmd job wait` has an overall timeout and a `GET` poll is still in
  flight at that deadline
- **THEN** the client MUST cancel that request at the overall deadline
- **AND** it MUST return a nonzero timeout outcome without waiting for the
  independent HTTP-client request timeout.
