// Package hosting owns one shared durable provider-period budget and opaque
// work/termination reservations. Reserve commits capacity before work and
// Release refunds only reserved capacity after its consumer joins that work.
// Sample shares bounded confirmed observations; it cannot authorize work.
// Hosting grants no token permission, placement or network authority.
// Linux Inspection separately retains complete existing period history and
// zero-reservation provenance under its own original lease. It mutates nothing
// and grants no current capacity; Close joins its original physical users.
//
// Navigation: one Budget owns the portable accounting clusters; the native
// inspection mechanism is instantiated per observation. Test naming follows
// docs/development/repository-layout.md.
//
// Responsibility clusters and their tests:
//   - allowance policy, directional cost and pressure accounting: allowance.go
//     (allowance_test.go)
//   - budget lifecycle, durable period root and committed journal: budget.go,
//     storage.go, failure.go (budget_linux_test.go; linux)
//   - work/termination reservation lifecycle and transfer deadlines:
//     reservation.go (Reserve/Release/DetachTermination),
//     reservation_request.go (reservation_linux_test.go,
//     deadline_linux_test.go, reservation_process_linux_test.go; linux)
//   - joint coverage and additional joint reservations: reservation.go
//     (CoversJoint/ReserveAdditionalJoint), joint_traffic.go
//     (joint_traffic_test.go, additional_reservation_linux_test.go)
//   - shared confirmed snapshot sampling: snapshot.go
//     (snapshot_linux_test.go; linux)
//   - Linux counter measurement and exclusive root lease: platform_linux.go,
//     platform_other.go, counter_file.go (counter_linux_test.go,
//     counter_file_test.go)
//   - retained read-only period inspection: inspection_linux.go
//     (inspection_native_linux_test.go; native linux)
package hosting
