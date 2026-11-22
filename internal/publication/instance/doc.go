// Package instance owns host-generated private Instance material and the exact
// canonical public request/response lifecycle. Public Credential verification
// does not expose a signer, root Authority key or accepting Publisher.
// Initialization claims only an empty root; retained openings require the
// complete current layout. Old, foreign or partial layouts are left unchanged,
// including missing lease or state files, never repaired into a fresh key.
// Original Publication lifetimes retain an immutable logical Service lease
// across Attachment gaps. Its purpose-specific TLS/proof users join physical
// JOIN cleanup before logical retirement and private Instance erasure.
// A replacement TLS borrow uses the same immutable lease and a pinned original
// continuity-key commitment. Purpose-specific response write remains setup
// bounded until separate transfer; Connection owns authentication, proposal
// accounting, byte-state validation and joined generation commit.
//
// Navigation: one consistency owner — Root.mu guards root, binding,
// recipient, lifetime, lease and attachment state, and private material never
// crosses the package boundary. The service_ file group is the Instance-side
// adapter of the Service Connection domain under that same lock. Test naming
// follows docs/development/repository-layout.md; all tests are package
// instance behavior tests, and the _linux tests require native custody.
//
//   - I1 root-lifecycle: root.go, generation_state.go, state_store.go,
//     response_lifecycle.go. Tests: lifecycle_linux_test.go,
//     root_refusal_linux_test.go, request_test.go.
//   - I2 native-custody: custody_linux.go, custody_other.go. Tests:
//     root_refusal_linux_test.go, lifecycle_linux_test.go.
//   - I3 grammar-adapter: request.go. Tests: request_test.go.
//   - I4 binding: binding.go, generation_consumption.go. Tests:
//     binding_linux_test.go, binding_coalescing_linux_test.go,
//     binding_observation_linux_test.go.
//   - I5 publication: publication_record.go, publication_lifetime.go. Tests:
//     publication_lifetime_linux_test.go, service_lifetime_test.go.
//   - I6 recipient: recipient.go, recipient_acceptance.go. Tests:
//     recipient_lifetime_test.go.
//   - I7 capsule-opening: capsule_opening.go. Tests: capsule_opening_test.go.
//   - I8 service: service_attachment.go, service_connection_lease.go,
//     service_initial_transfer.go, service_peer_close.go,
//     service_replacement.go, service_replacement_commit.go,
//     service_signing.go, service_tls_borrow.go. Tests:
//     service_lifetime_test.go, service_lifetime_linux_test.go,
//     service_certificate_test.go, service_signing_test.go.
package instance
