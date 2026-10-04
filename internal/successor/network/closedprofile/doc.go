// Package closedprofile owns the canonical, bounded ARDCPR03 signed profile
// grammar, preparation and verification. Its public token-key adapter delegates
// canonical SPKI validation to Admission's issuerprofile leaf.
// State alone joins verified entries to its current Epoch and commits acceptance.
package closedprofile
