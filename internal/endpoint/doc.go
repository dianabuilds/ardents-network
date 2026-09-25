// Package endpoint composes one role-local participant from authenticated
// State, Entry, Target, publication, and Route-Attachment owners. It implements
// the shared local Application Interfaces but owns no plan grammar, local
// transport grammar, Browser presentation, or Service Authority. A User Target
// lookup occurs only through an admitted Initiator operation; Endpoint never
// dials a resolution Gateway directly.
//
// Start at service_runtime.go for Endpoint construction and at text_context.go
// for participant admission, Job reservation, and the Context lock. The Context
// owns cross-owner admission and shutdown order; text_context_retirement.go
// lists the stop/join dependencies. The maintained lifecycle contract is in
// docs/technical/endpoint-service-runtime.md.
//
// The selected participant's private owners are grouped by responsibility:
//
//   - text_source_lifecycle.go and text_source_prefix.go own the exact Source
//     opening, handle, and retirement. text_source_operations.go owns the
//     serialized operation gate; text_source_set.go selects peers.
//   - text_introduction_prefix_lifecycle.go and
//     text_responder_prefix_lifecycle.go own the two Publisher prefixes.
//     text_introduction_admission.go, text_introduction_dispatch_state.go,
//     and text_introduction_exchange_set.go own opening rate, delivery slots,
//     and in-flight exchange membership under the Context lock.
//   - text_permission.go and text_permission_stock.go own holder authority and
//     issued stock; text_issuance_operation.go owns an admitted issuance attempt.
//   - text_publication_pair_lifecycle.go and text_publication_refresh.go own
//     publication registration and refresh; text_descriptor_publication.go
//     coordinates the signed Descriptor effect.
//   - descriptorhistory owns per-Context verified Descriptor floors;
//     text_resolution.go coordinates lookup and rechecks live authority.
//   - text_job_lifecycle.go owns invocation identity and joined cleanup;
//     text_worker_lifetime_linux.go owns the installed worker process lifetime.
//   - service_tls.go owns Instance authentication and exporter handoff;
//     protected_service_tls.go selects the protected Service groups and preserves
//     authenticated Route retirement through the TLS wrapper.
//
// The Route capsule package owns Introduction sealing and decoding; Endpoint
// checks decoded facts against live participant and publication authority.
//
// Durable token attempts and Transit Grant acquisition live in the child
// tokenjournal and transit packages. The durableroot package owns their shared
// filesystem lease and atomic-write primitives. The permissionfile package
// owns the separate owner-private offline permission file handover. Tests follow their production
// owner; the role-network fixture in text_issuance_network_test.go selects
// the Carrier, text_network_node_fixture_test.go owns each fixture Node's
// readiness and joined cleanup, and text_join_service_network_test.go exercises
// both selected Carriers.
package endpoint
