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
// Navigation: README.md carries the extended owner contract; the wire
// subpackage owns the closed v3 grammar and transcript proofs. Test naming
// follows docs/development/repository-layout.md; all tests are package
// connection behavior tests, and each test is listed under its primary
// responsibility cluster.
//
// Responsibility clusters and their tests:
//   - logical stream, attachment records and retirement: stream.go,
//     stream_records.go, attachment_record_reader.go (stream_test.go,
//     attachment_records_test.go, attachment_record_reader_test.go,
//     stream_retirement_seal_test.go, receive_delivery_test.go)
//   - bounded send replay ledger: send_ledger.go (send_ledger_test.go,
//     replacement_preflight_test.go)
//   - ordered receive/replay ledger: receive_ledger.go
//     (receive_ledger_test.go)
//   - terminal exchange and control tail: terminal_exchange.go,
//     terminal_tail.go (terminal_exchange_test.go,
//     terminal_replay_boundaries_test.go, terminal_tail_test.go)
//   - initial binding and instance verification: authenticated_attachment.go,
//     recipient_binding.go, initiator_binding.go, instance_certificate.go,
//     initial_flight.go, publisher_recipient.go
//     (authenticated_attachment_test.go, recipient_binding_test.go,
//     initiator_binding_test.go, instance_certificate_test.go,
//     initial_flight_test.go, exporter_transcript_test.go)
//   - two-role authentication and attachment lifetime:
//     initiator_authentication.go, initiator_attachment.go,
//     publisher_authentication.go (initiator_setup_test.go,
//     initiator_lifetime_test.go)
//   - initiator source seam: initiator_source.go (the sole
//     network/reachability/route seam) (initiator_source_test.go)
//   - finite recovery and replacement: recovery_episode.go,
//     publisher_recovery.go, initiator_replacement.go,
//     initiator_replacement_commit.go, publisher_replacement.go,
//     publisher_replacement_commit.go (recovery_episode_test.go,
//     publisher_replacement_commit_test.go,
//     publisher_replacement_retirement_test.go); the atomic ledger generation
//     transfer is verified by the replacement cases of send_ledger_test.go
//     and receive_ledger_test.go
//   - stream failure reason projection: stream_failure_linux.go
//     (stream_failure_linux_test.go; linux)
package connection
