// Package provisioning composes the offline production of signed infrastructure
// State inputs from protected operator files and independent public commitments.
// It owns no installed Node lifetime or Network State acceptance.
//
// Run in command_linux.go dispatches the offline role operations below;
// node_provisioning_role_linux.go owns their role schemas, operation names and
// refusal prose, and command_other.go holds the non-linux refusal stubs. The
// native role tests exercise this entry composition through pinned programs.
//
// Responsibility clusters and their tests:
//   - portable public preparation of detached unsigned bytes:
//     network_artifact_preparation.go, network_preparation_contract.go
//     (network_public_preparation_test.go)
//   - shared public identifier grammar: public_identifier_linux.go
//     (public_identifier_linux_test.go; linux)
//   - Node transport certificate codec: node_certificate_linux.go
//     (node_certificate_linux_test.go; linux)
//   - role Node Record declaration, signing and certificate command:
//     node_record_linux.go, node_provision_linux.go
//     (forwarding_node_record_linux_test.go; linux,
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
//     purpose declarations: network_signing_declaration_linux.go,
//     network_signing_contract.go, network_profile_signing_linux.go,
//     network_certificate_signing_linux.go, parseNetworkSigningKey in
//     network_signing_linux.go (network_signing_test.go,
//     network_profile_signing_test.go,
//     network_certificate_signing_linux_test.go; linux,
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
//     (network_preparation_native_linux_test.go; native linux)
//   - runtime plan preparation: runtime_plan.go, runtime_plan_command.go
//     (runtime_plan_command_test.go)
//
// network_receipt_fixture_test.go carries the shared portable preparation
// fixtures, native_fixture_linux_test.go the native protected-root and
// pinned-program fixtures, and forwarding_state_preparation_native_linux_test.go
// the full key/record/epoch/token/profile chain cohort consumed by
// forwarding_token_authority_native_linux_test.go; none has behavior of its own.
package provisioning
