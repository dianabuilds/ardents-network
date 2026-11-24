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
// A private Unix-family (Linux and macOS) file adapter opens, verifies, commits
// and releases the actual independently leased root: every mutation resolves
// through the held directory descriptor via the shared durablefile Openat /
// Renameat anchor and full-identity inode comparison, and a kernel root-move
// watch (Linux inotify, macOS kqueue EVFILT_VNODE) retains original
// move/loss/restoration evidence. Every other platform receives the same
// explicit typed refusal, never a weaker accepting path. Core callers cannot
// inject a persistence backend; encoding a snapshot grants no persistence or
// registration authority.
//
// Responsibility clusters and their tests:
//   - holder REGISTER/WITHDRAW lifetime and physical retirement:
//     registration.go (Register, Withdraw, read, stop, interruptPhysical,
//     Close, CleanupResult) (registration_test.go,
//     registration_retirement_test.go)
//   - holder idle supervision and observation scheduling: registration.go
//     (watch, checkOriginal, checkSupervision, observeOriginal)
//     (registration_supervision_test.go)
//   - holder Receipt/CheckReceipt public binding: registration_receipt.go
//     (registration_receipt_test.go)
//   - holder purpose-5 delivery children: registration_delivery.go
//     (registration_delivery_test.go)
//   - receiving REGISTER/RESULT/WITHDRAW exchange: receiving.go
//     (receiving_test.go)
//   - receiving delivery dispatch and physical join: receiving_delivery.go
//     (receiving_delivery_test.go)
//   - purpose-5 submission, source and receiving: submission.go,
//     receiving_submission.go (submission_test.go)
//   - canonical exchange wire codecs: registration_wire.go, delivery_wire.go
//     (registration_wire_test.go, delivery_wire_test.go)
//   - canonical slot snapshot codec: slot_snapshot.go (slot_snapshot_test.go)
//   - Registry transitions and portable History non-reclaim rules:
//     registry.go, slot_history.go (slot_history_test.go,
//     registry_unix_test.go; linux/darwin)
//   - Unix-family durable root lifecycle and crash/partial-write retention:
//     slot_history_unix.go (OpenHistory, InitializeHistory, commit,
//     replaceHistory, createStage, syncHistory) (slot_history_unix_test.go,
//     history_write_unix_test.go; linux/darwin,
//     history_crash_linux_amd64_test.go; linux/amd64)
//   - Unix-family custody verification: slot_history_unix.go (verify,
//     retainReplacement, retainSnapshot) (history_custody_unix_test.go;
//     linux/darwin)
//   - kernel root-move watch capability: slot_history_watch_linux.go
//     (inotify) and slot_history_watch_darwin.go (kqueue EVFILT_VNODE)
//   - unsupported-platform typed refusal: slot_history_other.go,
//     receiving_history_other.go (slot_history_other_test.go; !linux/!darwin)
//   - receiving registry composition: receiving_history_unix.go
//     (receiving_history_unix_test.go; linux/darwin)
//
// delivery_wire_test.go, receiving_test.go, registration_delivery_test.go and
// slot_history_unix_test.go also carry shared input fixtures
// (deliveryEnvelope, unadmittedRegistrationConn, deliveryTransportFixture,
// newHistoryFixture) used by other clusters; no file is fixture-only.
package introduction
