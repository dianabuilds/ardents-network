// Package custody owns the local encrypted seed record for a bounded release
// ceremony. Initialize creates the fixed release/control role set, and Inspect
// authenticates that record and returns its public receipt. No interface
// exposes a password, derived key, decrypted seed, signer, or signing operation
// to a caller.
package custody
