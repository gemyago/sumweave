## ADDED Requirements

### Requirement: Tag Hiding Honors Classification References
The finance catalog SHALL prevent internal tag hiding while any live classification rule references the tag and SHALL preserve historical transaction tag assignments.

#### Scenario: Referenced tag hiding is blocked
- **WHEN** a catalog tag-hide caller or a direct persistence save attempts to hide a tag referenced by a live classification rule
- **THEN** the operation MUST fail before changing the tag
- **AND** the domain failure MUST identify the referencing rule IDs through the catalog service
- **AND** the tag and all rule and transaction associations MUST remain intact.

#### Scenario: Unreferenced tag can be hidden
- **WHEN** an otherwise authorized hide operation targets a tag without live rule references
- **THEN** the system MUST logically hide the tag while retaining historical transaction assignments
- **AND** later rule create/update requests MUST reject that hidden tag.

#### Scenario: Rule changes release the hide restriction
- **WHEN** every rule referencing a tag is deleted or updated to remove that tag
- **THEN** the tag MUST cease to be blocked by classification references
- **AND** existing transaction tag assignments MUST remain unchanged.
