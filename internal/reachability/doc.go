// Package reachability verifies exact-Target private Descriptors. Receiving
// durable Store and local lookup history are separate Reachability owners, not
// authority obtained from a verified proof. Route owns the
// transport and current recipient selection; Publication owns delegated public
// proof authority. A verified Descriptor establishes no slot or Service readiness.
//
// Responsibility clusters and their tests:
//   - exact-Target Descriptor verification and currentness: descriptor.go
//     (descriptor_test.go)
//   - offline Descriptor draft preparation and signature completion:
//     descriptor_draft.go (descriptor_draft_test.go)
//   - local monotonic lookup history and retirement joins:
//     lookup_history.go (lookup_history_test.go)
//   - retained-floor comparison shared by Store publication and history:
//     retained_floor.go is exercised through store.go and lookup_history.go
//     below
//   - durable receiving Store publication, lookup, capacity and lifetime:
//     store.go, store_files.go (store_native_test.go,
//     retained_store_native_test.go — the retained opening never creates or
//     repairs — and store_lifetime_native_test.go — Close joins held work,
//     denial and recovery cannot revive a retired original; real filesystem
//     on all three platforms, store_crash_linux_amd64_test.go;
//     linux amd64 process crash at the actual persistence boundaries,
//     store_write_unix_test.go; unix partial-write retention)
//   - platform lease, directory sync and unsupported platform refusal:
//     store_platform_unix.go, store_platform_windows.go,
//     store_platform_other.go (store_lease_unix_test.go; unix lease loss
//     and replacement, store_platform_other_test.go; the unsupported
//     platform refuses before opening files)
//   - Store custody identity and Linux retained inspection: store_custody.go,
//     store_inspection_linux.go (store_inspection_native_linux_test.go and
//     store_inspection_join_native_linux_test.go; native)
//   - uncertain-stage repair: store_repair.go (store_repair_native_test.go;
//     real filesystem on all three platforms)
package reachability
