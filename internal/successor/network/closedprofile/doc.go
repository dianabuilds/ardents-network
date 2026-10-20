// Package closedprofile owns the canonical, bounded ARDCPR03 signed profile
// grammar, preparation, validated public signing transcript and verification.
// It receives no private key. Its public token-key adapter delegates
// canonical SPKI validation to Admission's issuerprofile leaf.
// State alone joins verified entries to its current Epoch and commits acceptance.
package closedprofile
