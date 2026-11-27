// Package connection owns one authenticated Service Connection under the
// original qualified Execution operation and verified Publication. The Target,
// Instance and ordered byte contract stay fixed across physical Attachments.
//
// Initial connection. The Reader calls InitiatorSource.OpenStream from
// endpoint/reader. initiator_source.go owns the retained Source and shared setup
// bounds. initiator_initial.go verifies the Descriptor, submits the capsule,
// opens the JOIN and hands off the initial authenticated Attachment.
// Publication runtime enters through NextStream and NextAuthentication.
// publisher_binding.go checks the recipient and publisher_authentication.go
// authenticates the original Responder JOIN with the non-exporting Instance.
// Both roles converge on the one-use openStream handoff in stream.go.
//
// Record exchange. recordio bounds the initial flights and the Stream actor's
// physical reads. wire owns the canonical records and transcripts. transfer
// owns ordered replay and Terminal ledgers under Stream.mu. stream_records.go
// matches each record effect to the exact Attachment's actors and generation.
// stream_io.go owns Application buffer handoff.
//
// Replacement. The Reader prepares a fresh JOIN and authenticates it through
// initiator_replacement_route.go and initiator_replacement.go. Publication
// dispatches a private replacement to PreparePublisherReplacement in
// publisher_replacement.go. publisher_route.go retains the exact Source and
// Responder scopes. stream_replacement.go owns both roles' common transition
// from failed actor pair to installed generation. Its commit checks the old
// pair joined before changing the active Attachment and replay state under
// Stream.mu. recovery_episode.go keeps the original deadline and proposal limit.
//
// Completion. Reader Source retirement and Publisher runtime retirement both
// reach stream_retirement.go. Stop seals effects immediately. Close joins
// setup, recovery and the current record actors before publishing the retained
// cleanup result. Terminal settlement uses the same lock and accepts an empty
// TLS truncation only after complete current control proofs and the exact
// decoded JOIN CLOSE from authenticated_attachment.go.
//
// Reader setup, Publisher replacement and Stream retirement tests exercise
// separate refusal and join boundaries. Endpoint's tagged TCP/TLS Connection
// and document-command profiles exercise the genuine Reader and Publisher.
package connection
