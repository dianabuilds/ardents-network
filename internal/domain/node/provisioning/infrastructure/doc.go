// Package infrastructure implements fixed-role offline provisioning under
// ardents infrastructure provision. Forwarding, JOIN and Descriptor each use
// their own declarations, key purpose and receipt namespace. They share the
// protected record, epoch, profile and token inventory mechanisms here.
// The Source genesis owner supplies its distinct one-record bundle workflow.
// The native protected effects require Linux. Unix-family pure role codecs
// retain their existing profile. Other hosts refuse the command before effects.
//
// Start at command_linux.go for dispatch and node_provisioning_role_unix.go
// for the closed role selection. node_record_unix.go and node_provision_linux.go
// carry Node key and Record preparation; epoch_provision_linux.go connects
// Epoch planning and signing. profile_plan_linux.go and profile_signing_linux.go
// join the verified Epoch and public Admission inventory to the Profile.
// token_material_linux.go produces the separate Admission inventory.
package infrastructure
