// Package protectedfiles retains a bounded set of original protected input files
// and their ancestor descriptors. Each acquisition has its own caller, failure
// and joined Close. It does not interpret declarations or grant Release,
// deployment, startup or runtime authority.
package protectedfiles
