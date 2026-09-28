// Package endpoint composes one role-local participant from authenticated
// State, Entry, Target, publication, and Route-Attachment owners. It implements
// the shared local Application Interfaces but owns no plan grammar, local
// transport grammar, Browser presentation, or Service Authority. A User Target
// lookup occurs only through an admitted Initiator operation; Endpoint never
// dials a resolution Gateway directly.
//
// Start at service_runtime.go for Endpoint construction and at duty_context.go
// for participant admission, Job reservation, and the Context lock. The Context
// owns cross-owner admission and shutdown order; duty_context_retirement.go
// lists the stop/join dependencies. The maintained lifecycle contract is in
// docs/technical/endpoint-service-runtime.md.
//
// The selected participant's private owners are grouped by responsibility:
//
//   - source_lifecycle.go and source_prefix.go own the exact Source
//     opening, handle, and retirement. source_operations.go owns the
//     serialized operation gate and retained Interior Set;
//     interior_set.go selects peers for Source and Publisher prefixes.
//   - introduction_prefix_lifecycle.go and
//     responder_prefix_lifecycle.go own the two Publisher prefixes.
//     introduction_admission.go, introduction_dispatch.go,
//     and introduction_exchange_set.go own opening rate, delivery routing,
//     and in-flight exchange membership under the Context lock.
//     introduction_recovery.go owns buffered recovery delivery, its
//     deadline refusal, and the join before waiter handoff or retirement.
//   - permission.go and permission_stock.go own holder authority and
//     issued stock; issuance_operation.go owns an admitted issuance attempt.
//   - publication_pair_lifecycle.go and publication_refresh.go own
//     the in-flight registration, withdrawal, publication pair, and refresh;
//     descriptor_publication.go coordinates the signed Descriptor effect.
//   - descriptorhistory owns per-Context verified Descriptor floors;
//     resolution.go coordinates lookup and rechecks live authority.
//   - job_lifecycle.go owns invocation identity and joined cleanup;
//     worker_lifetime_linux.go owns the installed worker process lifetime.
//     The child worker package owns the installed worker mechanism only —
//     artifact pinning, activation, process identity, credentialed attachment,
//     and verified stop — and grants nothing.
//   - service_tls.go owns Instance authentication and exporter handoff;
//     protected_service_tls.go selects the protected Service groups and preserves
//     authenticated Route retirement through the TLS wrapper.
//
// The Route capsule package owns Introduction sealing and decoding; Endpoint
// checks decoded facts against live participant and publication authority.
//
// Durable token attempts live in the child tokenjournal package. The
// durableroot package owns its shared filesystem lease and atomic-write
// primitives. The permissionfile package owns the separate owner-private
// offline permission file handover. Tests follow their production
// owner; the role-network fixture in issuance_network_test.go selects
// the Carrier, network_node_fixture_test.go owns each fixture Node's
// readiness and joined cleanup, and join_service_network_test.go exercises
// both selected Carriers.
package endpoint
