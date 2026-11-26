// Package prefix owns exact protected Entry/Interior physical generations,
// pending setup and sealed borrowing, original Source/Responder pair commits,
// retained original Source lifetimes and Responder scopes with exact joined physical-generation acquisitions,
// admitted terminal openings, finite issuer-only bootstrap, issuer and Descriptor exchanges,
// parent refill and joined resource return. Bootstrap never exposes admitted
// operations; Stock finalization follows physical join under the original opening.
// Reachability independently verifies Descriptor lookup and retains its private
// history after physical join, while the original opening still guards handoff.
// Consumers own Registration/JOIN operations and return claims after physical
// join. Prefix imports neither consumer; it opens no durable selection root,
// mints no Network or Admission authority and owns no Hosting budget.
//
// Responsibility clusters and their tests:
//   - closed-carrier adapter contract and physical retirement: the shared
//     route transport/TLS adapters composed by openPrefix in prefix.go;
//     the external tests drive those adapters directly
//     (adapter_contract_test.go, adapter_retirement_test.go)
//   - nested role record packing bounds: Entry/Interior nesting in prefix.go,
//     join_channel.go, introduction_channel.go; the test drives the shared
//     channel/role-TLS record preparation directly (record_packing_test.go)
//   - physical Entry/Interior opening, pending setup and joined retirement:
//     prefix.go, setup.go, opening.go (setup_test.go, retirement_test.go)
//   - background observation supervision: prefixObservation and the watch
//     timer in prefix.go (observation_test.go)
//   - sealed borrowing and original Source/Responder pair commits and
//     acquisitions: borrowing.go (borrowing_test.go)
//   - retained original Source lifetime: source_lifetime.go,
//     source_generation.go, source_join.go, source_operations.go,
//     source_scope.go (Prefix caller/scope rechecks) (source_lifetime_test.go)
//   - retained Responder scope: responder_scope.go, responder_join.go
//     (responder_scope_test.go)
//   - recipient resolution and exact rendezvous verification: recipient.go
//     (recipient_resolution_test.go)
//   - role lane opening and presentation shared by the issuer, Descriptor,
//     Introduction/Submission and JOIN terminals: role_lane.go
//     (role_lane_test.go)
//   - admitted terminal Introduction/Submission openings:
//     introduction_channel.go (introduction_channel_test.go)
//   - JOIN terminal channel setup and joined resource return: join_channel.go
//     (join_channel_test.go)
//   - finite issuer-only bootstrap and issuer exchange: bootstrap_prefix.go
//     (OpenBootstrap, BootstrapPrefix), issuer_exchange.go
//     (issuer_exchange_test.go)
//   - parent refill and role presentation composition: refill.go,
//     presentation.go, presentPrefixRole and bootstrapFlight in
//     bootstrap_prefix.go (presentation_test.go)
//   - Descriptor exchange and reachability lookup with private history:
//     descriptor_exchange.go (descriptor_response_test.go)
//
// physical_fixture_test.go carries the shared lifecycleConn physical-output
// and role TLS certificate fixtures and no behavior of its own.
package prefix
