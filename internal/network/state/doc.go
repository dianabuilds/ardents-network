// Package state orchestrates authenticated Network State acceptance and publication.
//
// Readers receive immutable snapshots only after durable publication. Source,
// clock, or resource-governor uncertainty prevents fresh publication. Its
// current bounded private encoding is not a public wire format. Its one
// ADR-0053 initialization operation creates only the separate encrypted
// functional-alpha Epoch authority and verifier-accepted empty genesis.
//
// The implementation follows the accepted decision from intake to readers:
// epoch verifies Epoch and Candidate View bytes; closedprofile verifies the
// separate signed profile grammar. State joins that profile to current Epoch
// candidates before durable acceptance. Refresh, selection, and offline_accept
// choose a current or pending decision; storage and control_* coordinate durable
// publication through the physical durable package. Snapshot_access, node_duty,
// resolution_view, and closed_profile_accept project copied current facts.
// Local Source role retention and collision checks stay with local_roles.
package state
