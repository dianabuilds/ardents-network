// Package duty owns bounded Endpoint-local Role Domain duty and exposure truth.
// Network supplies producer-scoped replacement, conflict, expiry and bound
// decisions; this adapter owns the exclusive root, journal and commit lifecycle.
// Its one current root format is version 2: ADR-0107 retired the historical
// receiving one-use Transit Grant spend ledger whose write operation died with
// the Route v2 execution closure (ADR-0093). A version-1 generation is still
// strictly decoded and converted in place - conflict duties, generation
// continuity, and watermark rollback protection are preserved while the spend
// records are dropped - but no writer emits version 1, and the first
// replacement commits version 2.
// Linux SourceInspection and NodeInspection independently retain existing
// read-only histories under their original leases. Source permits only retained
// exposure records; stopped Node inspection permits no producer record at all.
// Neither policy clears, repairs or initializes history, and native readers join
// before their original lease returns.
// Initialize is a separate explicit first-history operation for offline Source
// preparation. It claims only an existing vacant root with a newly born lease;
// existing or partial histories refuse without cleanup. Empty history grants no
// producer, exposure, Node duty or runtime right.
// Claim admits an absent producer without replacing retained records, including
// expired ones. Source uses it only at initial serving admission; its original
// runtime keeps Replace for updates and Remove after joined serving completion.
//
// Navigation: one store aggregate owns the portable clusters; the native
// inspection mechanism is instantiated per observation. Test naming follows
// docs/development/repository-layout.md; each test is listed under its primary
// responsibility cluster.
//
// Responsibility clusters and their tests:
//   - contract and bounded limits: contract.go (store_limits_test.go)
//   - root custody, lease and platform permissions: root.go,
//     root_lease_unix.go, root_lease_windows.go, root_permissions_unix.go,
//     root_permissions_windows.go, directory_sync_unix.go,
//     directory_sync_windows.go (retained_lease_unix_test.go,
//     retained_lease_windows_test.go, root_entries_test.go,
//     root_permissions_windows_test.go, store_release_failure_test.go with
//     the store_release_failure_unix_test.go and
//     store_release_failure_windows_test.go platform helpers)
//   - store operations, operation lease and conflict read: store.go,
//     operation_lease.go, conflict_read.go (store_test.go,
//     operation_lease_test.go, conflict_read_test.go,
//     conflict_concurrency_test.go, initialization_test.go)
//   - persistence and bounded v1 conversion: persistence.go, atomic_files.go
//     (v2 schema and the ADR-0107 v1 conversion) (legacy_conversion_test.go)
//   - participation policy adapter: participation_policy.go (the sole network
//     import), node_participation.go (node_participation_test.go,
//     source_collision_chain_test.go)
//   - node producer: node_producer.go (node_producer_test.go)
//   - retained history verification: retained_history.go (the repair-class
//     behavior is verified by the network/state repair tests; retained
//     opening by operation_lease_test.go above)
//   - native inspection (linux): inspection_linux.go,
//     source_inspection_linux.go, node_inspection_linux.go
//     (source_completion_policy_linux_test.go,
//     inspection_native_linux_test.go)
package duty
