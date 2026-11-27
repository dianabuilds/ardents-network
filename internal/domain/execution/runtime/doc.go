// Package runtime owns the original installed worker invocation. It admits the
// local session, pins and reobserves the worker, retains the qualified Grant,
// and joins the attachment, operation and pinned descendants. Local authority
// rules remain in execution and native worker mechanisms remain in worker.
//
// Start at the entry point for the required exchange:
//
//   - Document: LaunchReader and LaunchPublisher in document_reader_linux.go
//     and document_publisher_linux.go. document_protocol.go and document_init_*
//     bind the fixed inventory and INIT/READY exchange. The document handoff
//     files own the single Reader result and Publisher stream transfers.
//     Launch and Prepare use the same document inventory for Control and
//     preparation, with an observer that admits no post-READY Application bytes.
//   - Byte Stream: LaunchByteStreamReader and LaunchByteStreamPublisher in
//     byte_stream_linux.go bind Byte Stream INIT and its own observer. The
//     original qualified Operation transfers one pair of ports in byte_stream.go.
//   - Stream Qualification: stream_qualification_linux.go owns its fixed
//     inventory, INIT/READY and single transfer. The installed qualification
//     test is the only admitted caller.
//
// Each profile starts its observation once after common launch verification.
// Invocation retains the original Job and joins the observer before reporting
// the result. The Reader and qualification bridge share the authority-checked
// borrow in worker_borrow_unix.go. Platform refusal files preserve the portable
// entry points; actual installed worker coverage belongs to its Linux profile.
package runtime
