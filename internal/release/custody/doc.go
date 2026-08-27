// Package custody owns the local encrypted seed record for the bounded
// closed-alpha release ceremony. Its Initialize interface creates the fixed
// release/control role set and returns only public keys and an envelope digest.
// It never exposes a password, derived key, decrypted seed, or generic signing
// capability to a caller.
package custody
