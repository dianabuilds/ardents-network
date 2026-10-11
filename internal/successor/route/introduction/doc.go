// Package introduction owns holder REGISTER/WITHDRAW lifetime and separately
// receiving Introduction registration state and its
// independent durable non-reclaim history. It stores slot hashes and original
// expiries, never Target, recipient keys, capsule plaintext or Service authority.
// Admission remains responsible for token verification and irreversible spend.
// The receiving exchanges own REGISTER/RESULT/WITHDRAW and opaque purpose-5
// submission/delivery over the original registration, and recheck original
// role authority. Listener, Grant and physical resource retirement remain with
// receiving composition; holder and receiving lifetimes are distinct. Holder
// Registration consumes an opaque Prefix terminal and owns its exact request,
// reader, withdrawal attempt and joined result; it shares no live Registry root.
// Its immutable original-channel Receipt supplies public binding facts; each
// consuming effect checks live registration again through CheckReceipt.
// Receiving delivery retains finite pending users, channel-local nonces and
// whole-exchange byte debits through matching RESULT/CLOSE and physical join.
// It cannot decrypt a capsule or decide Publication/Connection acceptance.
// Source Submit consumes a fresh purpose-5/class-1 terminal under its exact
// retained duty, exclusions, caller and capsule deadline. Matching RESULT and
// joined TLS/lower termination precede final original authority checks.
// A receipt is neither Descriptor acknowledgement nor accepting Publication.
// Canonical slot snapshots, History rules and Registry transitions are portable.
// A private Linux file adapter opens, verifies, commits and releases the actual
// independently leased root. Core callers cannot inject a persistence backend;
// encoding a snapshot grants no persistence or registration authority.
package introduction
