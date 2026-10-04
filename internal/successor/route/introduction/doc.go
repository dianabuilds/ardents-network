// Package introduction owns receiving Introduction registration state and its
// independent durable non-reclaim history. It stores slot hashes and original
// expiries, never Target, recipient keys, capsule plaintext or Service authority.
// Admission remains responsible for token verification and irreversible spend.
package introduction
