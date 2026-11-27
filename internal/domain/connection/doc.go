// Package connection owns one authenticated Service Connection under the
// original qualified Execution operation and verified Publication. The Target,
// Instance and ordered byte contract stay fixed across physical Attachments.
//
// For the Reader, endpoint/reader opens an InitiatorSource and calls OpenStream.
// See initiator_source.go for the retained Source and initiator_initial.go for
// Descriptor verification, capsule submission, JOIN and the one-use Stream
// handoff. initiator_authentication.go authenticates the exact Instance over
// the original JOIN. For the Publisher, publication/runtime.NextStream calls
// NextAuthentication and then OpenPublisherStream. publisher_binding.go
// binds the private recipient and publisher_authentication.go authenticates
// the JOIN using the original non-exporting Instance lease.
//
// See recordio for the bounded initial flights and physical Stream reads, wire
// for the canonical records and transcripts, and transfer for ordered replay
// and the current Terminal exchange. stream_records.go joins the exact reader
// and writer for each Attachment and checks their generation before any
// ledger effect. stream_io.go owns Application buffer handoff.
//
// After physical loss, stream_recovery.go seals the original actors and starts
// the deadline in recovery_episode.go. initiator_replacement.go authenticates
// the Reader's replacement over the fresh JOIN from initiator_replacement_route.go.
// Publication dispatches a private replacement to PreparePublisherReplacement
// in publisher_replacement.go. replacement_commit.go handles both sides: the
// old actors and the new setup must join before replay and Attachment generation
// advance under Stream.mu. The same lock seals Stop and Close in
// stream_retirement.go, so retirement and a late proposal cannot both commit.
// terminal_tail.go checks the original decoded JOIN CLOSE and complete current
// Terminal proofs before permitting an empty TLS truncation.
//
// Setup, recordio, replacement and retirement tests cover the independent
// mechanics and causal refusals. The Endpoint Connection qualification
// profiles exercise the actual Reader and Publisher paths over TCP/TLS.
package connection
