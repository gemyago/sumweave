## ADDED Requirements

### Requirement: Bootstrap-First Styling Rails
Signal UI SHALL provide a Bootstrap-first styling contract for accepted V2 pilot pages so agents use framework classes and native HTML instead of bespoke route-local CSS.

#### Scenario: Pilot pages use vanilla Bootstrap classes
- **WHEN** an implementation touches a page covered by the Bootstrap pilot
- **THEN** the page MUST use vanilla Bootstrap CSS classes and native HTML or Svelte markup as the primary styling mechanism
- **AND** the implementation MUST NOT use a Svelte Bootstrap wrapper, a second utility CSS framework, or local design-system classes as the primary styling mechanism for that page
- **AND** canonical V1 routes MUST NOT be promoted to Bootstrap pilot behavior unless a later task explicitly approves that promotion

#### Scenario: Custom CSS is limited and documented
- **WHEN** a pilot page needs styling that Bootstrap classes cannot reasonably express
- **THEN** the custom CSS MUST be placed in a shared stylesheet rather than a route-local `<style>` block
- **AND** the rule MUST include a short comment naming the Bootstrap gap, shell containment need, third-party widget need, accessibility need, or browser fix it covers
- **AND** pilot pages MUST NOT use `style=` attributes for normal spacing, layout, color, typography, or component styling

#### Scenario: Agent rules enforce the styling contract
- **WHEN** the Bootstrap pilot is accepted for implementation
- **THEN** `apps/signal-ui/AGENTS.md` MUST tell agents to prefer Bootstrap classes and native HTML for pilot pages
- **AND** it MUST forbid new route-local bespoke CSS in pilot pages unless the exception is documented in the shared stylesheet
- **AND** `apps/signal-ui/DESIGN.md` MUST be updated so the Bootstrap pilot direction does not conflict with the visual design instructions agents are required to follow

### Requirement: V2 Route Isolation
Signal UI SHALL expose Bootstrap pilot pages under parallel V2 routes so the new styling rails can be tested without destabilizing canonical routes.

#### Scenario: V2 routes coexist with canonical routes
- **WHEN** the Bootstrap pilot routes are added
- **THEN** the UI MUST expose `#/v2/login` and `#/v2/finance`
- **AND** canonical `#/login` and `#/finance` MUST remain available and behaviorally unchanged by this change
- **AND** the app navigation MUST NOT redirect ordinary operators to V2 routes until a later explicit promotion decision is accepted

#### Scenario: V2 pages use clean route composition
- **WHEN** an operator opens a V2 pilot route
- **THEN** the route MAY use a V2-specific shell or composition boundary
- **AND** `#/v2/finance` MUST render inside a Bootstrap-specific protected shell that is distinct from `FinanceShell.svelte` and the generic authenticated `Nav` chrome
- **AND** that V2 finance shell MUST own tenant selection, sign-out and theme controls, and compact finance-local navigation for the pilot route
- **AND** the finance-local navigation MAY hand off to canonical `#/finance/*` destinations for non-pilot finance pages until later V2 finance routes are accepted
- **AND** V2 pilot routes MUST avoid importing V1 visual components whose primary purpose is to carry the old custom design system
- **AND** they MAY reuse behavior-only helpers such as `FinanceShellState`, API clients, auth stores, route guards, date formatting, and finance data utilities

### Requirement: Bootstrap V2 Login Pilot
The V2 login page SHALL prove the Bootstrap rails on a small public route without changing authentication behavior.

#### Scenario: Login renders with Bootstrap form patterns
- **WHEN** an unauthenticated operator opens `#/v2/login`
- **THEN** the page MUST render a Bootstrap-styled login form with labeled username and password controls, standard form validation or error placement, a primary submit button, and responsive centered layout
- **AND** the page MUST preserve existing successful-login routing, remembered protected destination behavior, disabled/loading submit behavior, and inline failure messaging
- **AND** the page MUST NOT contain route-local custom CSS for normal visual layout
- **AND** the canonical `#/login` page MUST remain available and unchanged until a later explicit promotion decision

#### Scenario: Login visual smoke passes on desktop and mobile
- **WHEN** the V2 login pilot implementation is verified
- **THEN** desktop and mobile browser checks MUST show no overlapping text, clipped controls, unreadable labels, console errors, or layout overflow
