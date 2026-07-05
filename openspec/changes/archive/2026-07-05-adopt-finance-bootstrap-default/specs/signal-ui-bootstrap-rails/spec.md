## ADDED Requirements

### Requirement: Canonical Bootstrap Login And Finance Landing
Signal UI SHALL use Bootstrap for the canonical login route and SHALL send default authenticated navigation to the Finance app.

#### Scenario: Canonical login uses Bootstrap and preserves auth behavior
- **WHEN** an unauthenticated operator opens `#/login`
- **THEN** the page MUST render a Bootstrap-styled login form with labeled username and password controls, standard form validation or error placement, a primary submit button, and responsive centered layout
- **AND** the page MUST preserve existing successful-login routing, remembered protected destination behavior, disabled/loading submit behavior, and inline failure messaging
- **AND** the page MUST NOT contain route-local custom CSS for normal visual layout

#### Scenario: Successful login lands on Finance by default
- **WHEN** an operator signs in successfully without a remembered protected destination
- **THEN** the UI MUST navigate to `#/finance`
- **AND** authenticated root/default navigation MUST also resolve to `#/finance`
- **AND** a remembered protected destination MUST still take precedence over the default Finance landing

## MODIFIED Requirements

### Requirement: Bootstrap-First Styling Rails
Signal UI SHALL provide a Bootstrap-first styling contract for canonical Finance app and login surfaces so agents use framework classes and native HTML instead of bespoke route-local CSS.

#### Scenario: Canonical Finance and login pages use vanilla Bootstrap classes
- **WHEN** an implementation touches `#/login` or a tenant-facing `#/finance*` route
- **THEN** the page MUST use vanilla Bootstrap CSS classes and native HTML or Svelte markup as the primary styling mechanism
- **AND** the implementation MUST NOT use a Svelte Bootstrap wrapper, a second utility CSS framework, or local design-system classes as the primary styling mechanism for that page
- **AND** non-finance routes MUST remain on the existing styling stack unless a later task explicitly accepts broader promotion

#### Scenario: Custom CSS is limited and documented
- **WHEN** a canonical Bootstrap Finance or login page needs styling that Bootstrap classes cannot reasonably express
- **THEN** the custom CSS MUST be placed in a shared stylesheet rather than a route-local `<style>` block
- **AND** the rule MUST include a short comment naming the Bootstrap gap, shell containment need, third-party widget need, accessibility need, or browser fix it covers
- **AND** canonical Bootstrap Finance and login pages MUST NOT use `style=` attributes for normal spacing, layout, color, typography, or component styling

#### Scenario: Agent rules enforce the canonical styling contract
- **WHEN** Bootstrap is accepted as the Finance app default
- **THEN** `apps/signal-ui/AGENTS.md` MUST tell agents to prefer Bootstrap classes and native HTML for canonical Finance and login pages
- **AND** it MUST forbid new route-local bespoke CSS in those pages unless the exception is documented in the shared stylesheet
- **AND** `apps/signal-ui/DESIGN.md` MUST be updated so the canonical Bootstrap Finance direction does not conflict with the visual design instructions agents are required to follow

## REMOVED Requirements

### Requirement: V2 Route Isolation
**Reason**: Bootstrap Finance and login are being promoted to canonical routes instead of remaining in a parallel pilot route space.
**Migration**: Use canonical `#/login` and `#/finance*` routes for product behavior; remove or retire pilot-only route expectations during implementation.

### Requirement: Bootstrap V2 Login Pilot
**Reason**: The login Bootstrap experience is no longer a pilot and is replaced by the canonical Bootstrap login requirement.
**Migration**: Use `#/login` for the Bootstrap login surface and preserve remembered-destination behavior there.
