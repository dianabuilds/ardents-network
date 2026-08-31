// Package endpoint composes one role-local participant from authenticated
// State, Entry, Target, publication, and Route-Attachment owners. It implements
// the shared local Application Interfaces but owns no plan grammar, local
// transport grammar, Browser presentation, or Service Authority. A User Target
// lookup occurs only through an admitted Initiator operation; Endpoint never
// dials a resolution Gateway directly.
package endpoint
