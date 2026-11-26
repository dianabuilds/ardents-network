// Package node owns one original Forwarding, Issuer, Descriptor, Introduction or JOIN invocation: startup acquisitions,
// synchronous denial, physical retirement and retained completion. Network,
// Admission, Hosting, Reachability, Route and processguard retain their separate policies.
//
// The original invocation declaration grammar (plan_declaration.go) is pure
// configuration validation kept with this runtime owner (ADR-0125 decision 6): a
// closed byte codec mirroring network/source/servingdeclaration.ServingDeclaration, shared by the
// runtime, the retained repair commands and the node prepare-plan operator
// command. It returns declaration facts only and opens no resource. It performs
// no State acceptance, root creation, hosting initialization, signing or
// processguard policy. Node validates invocation root separation, and the
// command binds the declared State and selected role constructor. After startup,
// Node supervises the original invocation and releases borrowed authority only
// after physical join. The shared declaration and supervision mechanisms leave
// each role's operational policy with its owner.
//
// Responsibility clusters and their tests:
//   - invocation supervision and completion timing: invocation.go
//     (invocation_test.go)
//   - failure projection and phase attribution: failure_projection.go
//     (failure_projection_test.go, failure_projection_unix_test.go; unix)
//   - admission fact projection of the original network observation:
//     admission_projection.go (admission_projection_test.go)
//   - declaration grammar, root independence and repair binding:
//     plan_declaration.go, repair_declaration.go, root_paths.go
//     (plan_declaration_test.go, repair_declaration_test.go,
//     root_paths_test.go)
//   - role command composition: command.RunRole reads the plan, owns State
//     and renders the outcome. The Plan Start methods open the role runtime.
//   - runtime credential custody: internal/credential owns loading, JOIN
//     file pins and erasure. Each role owns the membership comparison and
//     joins TLS users before returning its material.
//   - role runtime lifetime and retirement: forwarding_unix.go,
//     issuer_unix.go, descriptor_unix.go, introduction_unix.go,
//     join_unix.go, lifetime_unix.go, runtime_unix.go, runtime_other.go
//     (descriptor_startup_unix_test.go, introduction_startup_unix_test.go,
//     join_startup_unix_test.go, lifetime_equivalence_unix_test.go,
//     retirement_unix_test.go; unix)
//   - route receive and admission authority: route_admission_linux.go,
//     route_receive_linux.go, route_receive_other.go,
//     forwarding_admission_unix.go, forwarding_observation_unix.go,
//     issuer_reservation_unix.go
//     (route_bootstrap_linux_test.go, route_bootstrap_capacity_linux_test.go,
//     route_descriptor_linux_test.go, route_join_linux_test.go,
//     route_join_lifecycle_linux_test.go, route_refill_linux_test.go,
//     route_refusal_linux_test.go, route_registration_linux_test.go,
//     route_submission_linux_test.go, route_token_linux_test.go; linux,
//     shared fixtures route_receiving_fixture_linux_test.go,
//     route_join_fixture_linux_test.go and
//     publication_registration_linux_test.go)
//   - native Linux process, authority and TCP evidence for all five roles:
//     (authority_process_native_linux_test.go,
//     operation_process_native_linux_test.go,
//     operation_workload_native_linux_test.go,
//     introduction_exchange_native_linux_test.go,
//     refill_process_native_linux_test.go,
//     forwarding_observation_native_linux_test.go; native)
package node
