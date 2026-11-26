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
//     independent signing inputs; network_preparation_contract.go owns the
//     completed artifact receipt.
//   - shared public identifier grammar: public_identifier_unix.go
//     (public_identifier_unix_test.go; unix)
//   - Node transport certificate codec: node_certificate_unix.go
//     (node_certificate_unix_test.go; unix)
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
//   - Network signing admission and fixed record/epoch/profile/certificate
//     purpose declarations: network_signing_declaration_unix.go,
//     network_signing_contract.go, network_profile_signing_unix.go,
//     network_certificate_signing_unix.go, parseNetworkSigningKey in
//     network_signing_linux.go (network_signing_test.go; linux,
//     network_profile_signing_test.go,
//     network_certificate_signing_unix_test.go; unix,
//     network_profile_native_linux_test.go; native linux)
//   - SignNetwork operation lifetime and original input custody: SignNetwork
//     in network_signing_linux.go, network_signing_other.go
//     (network_signing_custody_native_linux_test.go; native linux)
//   - signed inventory publication durability: PublishNetworkInventory in
//     network_output_linux.go (network_inventory_native_linux_test.go;
//     native linux)
//   - unsigned artifact preparation and publication durability:
//     network_preparation_linux.go, network_preparation_other.go,
//     PublishPreparedNetworkArtifact in network_output_linux.go
//     (network_preparation_native_linux_test.go; native linux). The input
//     reader uses the Unix-family durablefile inspection port (#850), but the
//     operation keeps its Linux boundary because publication composes the
//     Linux ext4 output floor.
//
// network_receipt_fixture_test.go carries the shared portable preparation
// fixtures, network_signing_fixture_unix_test.go the shared Unix-family signing
// fixtures, native_fixture_linux_test.go the native protected-root and
// pinned-program fixtures, and forwarding_state_preparation_native_linux_test.go
// the full key/record/epoch/token/profile chain cohort consumed by
// forwarding_token_authority_native_linux_test.go; none has behavior of its own.
package provisioning
