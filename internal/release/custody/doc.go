// Package custody owns the local encrypted seed record for the bounded
// closed-alpha release ceremony. Initialize creates the fixed release/control
// role set; Inspect authenticates that record and returns its public receipt;
// BuildAlphaInputs creates only the fixed verifier-preflighted ADR-0052 static
// directory. No interface exposes a password, derived key, decrypted seed, or
// generic signing capability to a caller.
package custody
