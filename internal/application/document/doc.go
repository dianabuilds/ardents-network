// Package document owns the bounded file transfer over the common Service byte stream, trusted
// import of an explicitly selected local snapshot, and safe UTF-8 presentation.
// The historical package name also covers binary file snapshots. No text
// protocol or network content-type selection is implemented here.
// It owns no destination or publication authority. Its worker entrypoints have
// no file-selection operation and receive only an Endpoint-supplied snapshot.
//
// Responsibility clusters and their tests:
//   - bounded client document read: exchange.go (exchange_test.go)
//   - safe UTF-8 presentation: presentation.go (presentation_test.go)
//   - safe read failure messages: read_failure_message.go
//     (read_failure_message_test.go, read_failure_message_linux_test.go)
//   - trusted local snapshot file import: snapshot_file_unix.go (Linux and
//     macOS), snapshot_file_other.go (explicit refusal on other platforms)
//     (snapshot_file_unix_test.go)
//   - worker initialization and readiness binding:
//     worker_attachment_initialization.go, readWorkerInitialization and
//     writeWorkerReadiness in worker_initialization.go
//     (worker_attachment_initialization_test.go)
//   - fixed worker multiplexer, scheduling and frame codecs: worker.go,
//     worker_scheduling.go, worker_mode.go, worker_output.go,
//     readWorkerFrame/writeWorkerFrame/writeWorkerBytes in
//     worker_initialization.go (worker_test.go)
//   - byte-stream worker relay and host bridge: worker_stream_relay.go,
//     worker_stream_attachment.go (worker_stream_relay_test.go)
//   - publisher connection serving and stream I/O: publisher_connections.go,
//     publisher_connection_io.go, publisher_connection_events.go
//     (publisher_connections_test.go,
//     publisher_connection_fragmentation_test.go)
//   - publisher admission bounds and failure accounting:
//     publisherOpenLimit/publisherActiveLimit in publisher_connections.go,
//     failStream and releaseCredit in publisher_connection_events.go,
//     cleanup-error join in publisher_connection_io.go
//     (publisher_connection_limits_test.go)
//   - publisher hostile-worker refusal and cancellation ownership:
//     fromWorker validation in publisher_connection_events.go,
//     readWorkerOutput in worker_output.go
//     (publisher_connection_refusal_test.go)
//   - publisher original control-tail retention: startClose and finish in
//     publisher_connection_io.go (publisher_control_tail_test.go)
//   - reader connection exchange and result validation: reader_connection.go,
//     reader_connection_io.go (reader_connection_test.go,
//     reader_connection_fragmentation_test.go,
//     reader_connection_aai3_linux_test.go; linux)
//   - reader retained transfer and single-stream handoff:
//     AwaitRetainedWorkerConnection and awaitWorkerConnection in
//     reader_connection.go (reader_transfer_test.go)
//   - reader refusal and cancellation join: fromWorker/fromResult guards in
//     reader_connection.go, readerConnectionIO.close in reader_connection_io.go
//     (reader_connection_refusal_test.go)
//   - installed worker inherited-descriptor audit: worker_process_linux.go
//     (verifyWorkerDescriptors, managerCgroupControlTarget,
//     descriptorSurvivedExec), worker_process_other.go
//     (worker_descriptor_audit_linux_test.go; linux,
//     worker_descriptor_audit_native_linux_test.go; native linux)
//   - installed worker confinement qualification: RunInheritedWorker in
//     worker_process_linux.go, RunWorker in worker.go
//     (worker_escape_linux_test.go, worker_hostile_linux_test.go;
//     native linux TestMain-only pinned artifacts)
//
// reader_connection_fixture_test.go carries the linux snapshot-connection
// fixture serving reader_connection_aai3_linux_test.go and no behavior of its
// own.
package document
