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
// operation seam. The current vertical slice also exports a Bundle only to a
// new Owner-selected destination and isolatedly test-restores it. Restore
// creates an encrypted authority-locked export-only record in a separate
// quarantine root. Bundle replacement is a separately confirmed owner action:
// the prior encrypted bundle is restored on failed final publication or test
// restore. Active records advance one durable non-decreasing local
// Authority floor only after their encrypted record is published, and active
// verification requires an exact matching floor. A locked recovered Name
// Authority activates only from a fresh opaque witness of one current active
// Namespace record that is strictly higher than the recovered generation and
// revision; custody advances its local watermarks, durably writes the active
// successor and floor. Service Authority creation instead generates its root
// inside custody and returns only the public Authority and derived Target; it
// creates neither a runtime Instance Key nor a Grant.
package custody
