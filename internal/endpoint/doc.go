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
//   - source.Lifecycle owns the exact Source opening, handle, retirement,
//     and serialized operation reservation. source_operations.go coordinates
//     Context admission and Source use; the root retains the Interior Set,
//     and interior_set.go selects peers for Source and Publisher prefixes.
//   - introduction.go composes the prefix, admission, dispatch, and exchange
//     owners under the Context lock. introduction/admission.go,
//     dispatch_state.go, exchange_set.go, recovery.go, and pair_lifecycle.go
//     own their respective mechanisms; introduction_prefix_lifecycle.go and
//     responder_prefix_lifecycle.go own the two Publisher prefixes. Root
//     introduction_registration.go and introduction_receive.go coordinate
//     registration and delivery with the other Context owners.
//   - permission.go checks the live State profile. The tokens package owns
//     holder permission, issued stock, and the admitted operation;
//     issuance.go coordinates Source use and token issuance.
//   - publication.go composes the registration pair and refresh scheduler
//     with Publisher startup and drain. introduction/pair_lifecycle.go owns
//     current, pending, and previous registrations; publication/refresh.go
//     owns scheduler identity. publication_refresh.go coordinates rotation,
//     and descriptor_publication.go coordinates the signed Descriptor effect.
//   - descriptorhistory owns per-Context verified Descriptor floors;
//     resolution_lifecycle.go owns the single lookup-or-publication flight,
//     its exact Source acquisition, cancellation join, and completion.
//     resolution.go coordinates lookup and rechecks live authority.
//   - job_lifecycle.go owns invocation identity and joined cleanup;
//     worker_lifetime_linux.go owns the installed worker process lifetime.
//     The child worker package owns the installed worker mechanism only —
//     artifact pinning, activation, process identity, credentialed attachment,
//     and verified stop — and grants nothing.
//   - service_binding.go retains job and publication authority. The service
//     package owns TLS Instance authentication, protected Service attachment,
//     workload and stream mechanics; service_route_recovery.go coordinates
//     Route recovery across the binding seam.
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
