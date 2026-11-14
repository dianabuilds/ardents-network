// Package instance owns host-generated private Instance material and the exact
// canonical public request/response lifecycle. Public Credential verification
// does not expose a signer, root Authority key or accepting Publisher.
// Initialization claims only an empty root; retained openings require the
// complete current layout. Old, foreign or partial layouts are left unchanged,
// including missing lease or state files, never repaired into a fresh key.
// Original Publication lifetimes retain an immutable logical Service lease
// across Attachment gaps. Its purpose-specific TLS/proof users join physical
// JOIN cleanup before logical retirement and private Instance erasure.
// A replacement TLS borrow uses the same immutable lease and a pinned original
// continuity-key commitment. Purpose-specific response write remains setup
// bounded until separate transfer; Connection owns authentication, proposal
// accounting, byte-state validation and joined generation commit.
package instance
