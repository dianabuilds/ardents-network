// Package connection owns immutable logical/Attachment bindings, initial
// two-role Service authentication and logical byte/Terminal state under the
// original qualified Execution operation and independently verified Publication.
// A copied binding alone grants no authenticated stream or recovery.
// Private protocol mechanisms validate the closed v3 grammar, exact Instance
// certificate/proof and exporter transcripts, bounded send replay and ordered
// receive/replay storage, finite recovery proposals and directional Terminal
// receipt/confirmation ordering.
// A completed control tail consumes only exact decoded JOIN CLOSE evidence
// for an empty TLS truncation; it retains logical checks and physical joins.
// Those mechanisms do not supply live authority or an authenticated stream.
// Publisher authentication consumes purpose-specific non-exporting Instance
// TLS/proof; Reader authentication independently verifies that exact Instance.
// Their sealed initial state has one logical-stream consumer with an original
// checked workload contract. Publication routes fresh private recipients to
// the same Publisher Stream; InitiatorSource re-resolves the original Target
// through its retained History and Rendezvous set. Both roles retain finite
// original-key proposals and join old actors before atomic replay generations.
// Protected Application command consumers use those sources before Application handoff;
// complete native/installed acceptance remains a separate evidence boundary.
//
// For the Reader, start at initiator_source.go. It binds the original Target
// and retained Prefix. initiator_authentication.go checks the initial Proof and
// Continuity. The private instancetls package checks the exact Instance key
// for both initial and replacement TLS. publisher_authentication.go checks the
// independently admitted recipient. wire owns the record and transcript bytes.
//
// stream.go consumes the initial proof once and retains the logical owner.
// stream_io.go owns caller-memory handoff and ordered writes. send_ledger.go,
// receive_ledger.go and terminal_exchange.go retain the directional offsets and
// receipts. stream_records.go serves one exact physical Attachment generation.
//
// recovery_episode.go retains the original recovery bound. The initiator and
// Publisher prepare separate proposals. replacement_commit.go advances all
// three ledgers only after the original physical actors and setup have joined.
// stream_retirement.go preserves the original result through terminal_tail.go's
// separate control tail and physical join. The stream, replacement and
// terminal tests exercise original, foreign and late paths.
package connection
