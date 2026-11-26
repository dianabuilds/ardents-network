// Package provisioning composes the offline production of signed infrastructure
// State inputs from protected operator files and independent public commitments.
// It owns no installed Node lifetime or Network State acceptance. Runtime
// declaration output belongs to the separate runtimeplan command package.
//
// Run in command_linux.go dispatches the offline role operations below;
// node_provisioning_role_unix.go owns their role schemas, operation names and
// refusal prose, and command_other.go holds the non-linux refusal stubs. The
// native role tests exercise this entry composition through pinned programs.
//
// Responsibility clusters and their tests:
//   - networkdraft owns the public declaration grammar and unsigned bytes.
//     network_public_preparation_test.go checks the prepared bytes against
//     independent signing inputs.
//   - shared public identifier grammar: public_identifier_unix.go
//     (public_identifier_unix_test.go; unix)
//   - Node transport certificate codec: networksigning/certificate_unix.go
//     (networksigning/certificate_unix_test.go; unix)
//   - role Node Record declaration, signing and certificate command:
//     node_record_unix.go, node_provision_linux.go
//     (forwarding_node_record_unix_test.go; unix,
//     node_provision_native_linux_test.go; native linux)
//   - role Epoch plan and genesis signing: epoch_plan_linux.go,
//     epoch_signing_linux.go, epoch_provision_linux.go
//     (forwarding_epoch_plan_linux_test.go; linux,
//     epoch_provision_native_linux_test.go; native linux)
//   - role closed-profile preparation and signing: profile_plan_linux.go,
//     profile_signing_linux.go (profile_native_linux_test.go; native linux)
//   - offline token-key material request: token_material_linux.go
//     (forwarding_token_material_linux_test.go; linux,
//     token_material_native_linux_test.go,
//     forwarding_token_authority_native_linux_test.go; native linux); the
//     authority test composes the produced inventory through genuine State
//     acceptance and Custody Admission stock
//   - role key birth: provisioning_key_linux.go, the prepare-node-key branch
//     of node_provision_linux.go and the prepare-state-key branch of
//     epoch_provision_linux.go (provisioning_key_native_linux_test.go;
//     native linux)
//   - Source genesis control: source_initial_plan_linux.go,
//     source_initial_network_linux.go, source_initial_keys_linux.go
//     (source_initial_network_linux_test.go; linux,
//     source_initial_network_native_linux_test.go; native linux)
//   - initial Network key birth: initial_network_keys_linux.go
//     (initial_network_keys_linux_test.go; linux,
//     initial_network_keys_native_linux_test.go; native linux)
//   - networksigning owns the four fixed signing purposes, checks and protected
//     file lifetime. This package dispatches arguments and writes its receipt.
//     Linux custody and output tests exercise the signing package through the
//     registered native profile.
//   - networkpreparation owns original public file admission and exclusive
//     unsigned output. Its Linux publication tests cover filesystem failures
//     under the ext4 output floor; other platforms refuse before effects.
//
// The independent public and signing inputs are in tests/networkfixture.
// native_fixture_linux_test.go retains the native protected-root and
// pinned-program fixtures, and forwarding_state_preparation_native_linux_test.go
// the full key/record/epoch/token/profile chain cohort consumed by
// forwarding_token_authority_native_linux_test.go; none has behavior of its own.
package provisioning
