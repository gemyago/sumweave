## MODIFIED Requirements

### Requirement: Direct Client Enforces Transport Safety

The direct client SHALL use bounded requests and normal certificate verification
for every remote API call.

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

#### Scenario: Overall wait deadline bounds an in-flight poll

- **WHEN** `swmd job wait` has an overall timeout and a `GET` poll is still in
  flight at that deadline
- **THEN** the client MUST cancel that request at the overall deadline
- **AND** it MUST return a nonzero timeout outcome without waiting for the
  independent HTTP-client request timeout.
