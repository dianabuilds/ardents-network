// Package custody owns password-protected Authority Vault records and Recovery
// Bundles. Its interface never returns a password, derived key, decrypted root
// material, or signing capability. Callers submit one bounded custody operation
// and receive only a classified receipt.
//
// The package implements the accepted ardents-authority-envelope-v1 profile:
// a fixed Argon2id derivation followed by AES-256-GCM with a fresh random nonce.
// It validates the canonical envelope and every fixed parameter before starting
// an expensive derivation. The current vertical slice creates independent Vault
// records, verifies their semantic binding, and inspects unencrypted envelope
// headers; Bundle export, restore, and reconciliation extend the same custody
// operation seam.
package custody
