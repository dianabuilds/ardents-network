// Package signing produces purpose-specific offline infrastructure Release
// artifacts. It verifies its outputs through the independent Release verifier.
//
// Responsibility clusters and their tests:
//   - offline command entry and role dispatch: command_linux.go,
//     command_other.go (the non-Linux typed refusal) are exercised end-to-end
//     through the built command by the native evidence clusters below
//   - public-option grammar and operator argument parsing:
//     node_release_arguments_linux.go (node_release_arguments_linux_test.go)
//   - release plan grammar and purpose separation: node_release_plan_linux.go,
//     source_release_plan_linux.go (descriptor_release_plan_linux_test.go,
//     forwarding_release_plan_linux_test.go, join_release_plan_linux_test.go,
//     source_release_plan_linux_test.go; linux — the last file also pins the
//     shared artifactio private-key parser's caller-input retention used by
//     the key commands)
//   - release key preparation and ordered public receipt:
//     descriptor_release_keys_linux.go, forwarding_release_keys_linux.go,
//     join_release_keys_linux.go, source_release_keys_linux.go,
//     infrastructure_release_keys_linux.go, release_key_receipt_unix.go
//     (release_key_receipt_unix_test.go; unix,
//     release_key_receipt_native_linux_test.go; native)
//   - Forwarding/JOIN/Descriptor initial and forward Release signing with
//     predecessor authentication: node_release_linux.go,
//     node_release_predecessor_linux.go
//     (descriptor_release_native_linux_test.go,
//     descriptor_forward_release_native_linux_test.go,
//     forwarding_release_native_linux_test.go,
//     forwarding_forward_release_native_linux_test.go,
//     join_release_native_linux_test.go,
//     join_forward_release_native_linux_test.go; native, shared root/ext4
//     parent fixture native_fixture_linux_test.go)
//   - Source Release signing, timestamp refresh and cross-generation
//     coherence: source_release_linux.go, source_release_predecessor_linux.go
//     (source_release_native_linux_test.go,
//     source_release_timestamp_native_linux_test.go,
//     source_release_coherence_native_linux_test.go; native)
package signing
