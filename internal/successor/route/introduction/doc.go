// Package introduction owns holder REGISTER/WITHDRAW lifetime and separately
// receiving Introduction registration state and its
// independent durable non-reclaim history. It stores slot hashes and original
// expiries, never Target, recipient keys, capsule plaintext or Service authority.
// Admission remains responsible for token verification and irreversible spend.
// The receiving exchange owns REGISTER/RESULT/WITHDRAW and rechecks original
// role authority. Listener, Grant and physical resource retirement remain with
// receiving composition; holder and receiving lifetimes are distinct. Holder
// Registration consumes an opaque Prefix terminal and owns its exact request,
// reader, withdrawal attempt and joined result; it shares no live Registry root.
// Canonical slot snapshots, History rules and Registry transitions are portable.
// A private Linux file adapter opens, verifies, commits and releases the actual
// independently leased root. Core callers cannot inject a persistence backend;
// encoding a snapshot grants no persistence or registration authority.
package introduction
