// Package state owns authenticated Network State admission, transition, and
// publication. Open verifies retained current, pending, and distribution state
// before starting any Source server or background work. Accept admits a
// current offline decision; Refresh resolves finite Source waves and may
// stage a pending one. Both obey the current/pending/conflict invariant and
// publish immutable decisions through the physical durable root.
//
// Epoch authenticates candidate bytes; Source owns finite TLS acquisition and
// distribution framing. State alone selects and commits the accepted decision.
// Closedprofile verifies the separate signed profile grammar, while State
// joins it to the current Epoch before exposing closed Route facts. Duty holds
// local Source exposure and serving-role conflicts. Process constraints are
// borrowed through PermitWork; Network owns no process-resource monitor.
// Readers receive copied views,
// and Wait and Close supervise and join the work owned by this State root.
//
// CurrentRuntime observes trusted time and joins the accepted profile
// with exact Node Records under one read lock. Its opaque view owns copied
// facts, rejects its zero value, and binds retained local duties to the signed
// identity, generation, family, assignment and Carrier. Member validity is
// separate from profile validity. Consumers retain Purpose, path selection and
// transport ownership, and must query the live State again at admission
// rechecks; retaining a view does not acquire a lease on future acceptance.
package state
