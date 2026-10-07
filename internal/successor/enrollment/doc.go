// Package enrollment authenticates one initial portable bundle against an
// independently delivered manifest pin. It retains exact verified bytes;
// it grants no Release, installation, execution or Network authority.
// ReadCandidate separately retains self-consistent untrusted bytes without
// first-pin or running-program provenance for a later Release evaluation.
// ValidateProtectedGeneration checks the static descriptor/resource grammar
// for Installation's root-controlled inspection without minting pin provenance.
package enrollment
