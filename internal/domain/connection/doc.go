// Package connection owns one authenticated Service Connection under the
// original qualified Execution operation and verified Publication. The Target,
// Instance and ordered byte contract stay fixed across physical Attachments.
//
// Reader. reader_source.go owns the retained Source, selects the Descriptor,
// submits the capsule, opens the initial JOIN and hands off to Stream.
// reader_authentication.go prepares the Reader binding and verifies the exact
// Instance through TLS and the initial proof. reader_attachment.go retains the
// physical JOIN and its TLS borrowers. reader_replacement.go owns one recovery
// proposal from the fresh route acquisition through authenticated handoff.
//
// Publisher. recipient_binding.go binds the original qualified operation.
// publisher_authentication.go authenticates the Responder JOIN. publisher_route.go
// retains the exact Source and Responder scopes. publisher_replacement.go
// verifies the candidate for the same stream and prepares its replacement.
//
// Both roles check the capsule in recipient_binding.go and derive their public
// Publication and transcript commitments in recipient. Both converge on the
// one-use openStream handoff in stream.go.
//
// Record exchange. recordio bounds the initial flights and the Stream actor's
// physical reads. wire owns the canonical records and transcripts. transfer
// owns ordered replay and Terminal ledgers under Stream.mu. stream_records.go
// matches each record effect to the exact Attachment's actors and generation.
// stream_io.go owns Application buffer handoff.
//
// Replacement commit. stream_replacement.go joins the previous record pair
// before installing the new Attachment and replay state under Stream.mu.
// transfer/recovery_episode.go keeps the original deadline and proposal limit.
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
