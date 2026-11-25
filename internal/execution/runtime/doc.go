// Package runtime orders genuine installed worker launch, readiness, exact
// local permission and joined physical cleanup. Local execution rules remain
// in execution; fixed native mechanisms remain in worker, and document INIT belongs
// to document. Preparation provenance never authorizes a dead worker.
// A qualified Publisher operation retains its original admitted Administration
// context separately from the worker lease and Job's joined completion.
//
// Navigation: one launch flow (launchInitializedJob) parameterized by a
// private workerProtocol; Operation, Invocation and Owner share unexported
// concurrency state. Test naming follows
// docs/development/repository-layout.md. Installed behavior tests require the
// native Linux worker; the _other refusal tests run on every platform.
//
//   - ER1 owner-launch: launch.go, invocation.go, preparation.go, protocol.go.
//     Tests: launch_test.go, preparation_test.go, invocation_retirement_test.go,
//     native_refusal_other_test.go; installed:
//     preparation_installed_linux_test.go,
//     repeated_launch_installed_linux_test.go,
//     endpoint_death_installed_linux_test.go, escape_installed_linux_test.go,
//     hostile_tree_installed_linux_test.go.
//   - ER2 operation: operation.go, connection_context.go, stream_contract.go,
//     administration_context.go, administration_request.go. Tests:
//     operation_test.go, stream_contract_test.go,
//     administration_context_test.go, administration_request_test.go;
//     connection_context.go is also exercised by
//     preparation_installed_linux_test.go and reader_installed_linux_test.go.
//   - ER3 reader-worker: reader_worker_linux.go, reader_worker_other.go,
//     reader_stream.go. Tests: reader_worker_other_test.go,
//     reader_stream_test.go, reader_installed_linux_test.go.
//   - ER4 publisher-worker: publisher_worker_linux.go,
//     publisher_worker_other.go, publisher_stream.go. Tests:
//     publisher_worker_linux_test.go, publisher_installed_linux_test.go.
//   - ER5 stream-qualification: stream_qualification_linux.go,
//     stream_qualification_other.go. Tests:
//     stream_qualification_installed_linux_test.go.
//   - ER6 init-seam: initialization_linux.go, initialization_other.go;
//     exercised through the installed launch and native-refusal tests.
//   - ER7 byte-stream: byte_stream.go (portable ExchangeByteStream),
//     byte_stream_linux.go (Linux launch and protocol), byte_stream_other.go
//     (non-Linux LaunchByteStreamPublisher/LaunchByteStreamReader refusal stubs,
//     #866). The authority-rechecking borrow shared by the Reader and Stream
//     qualification bridges is worker_borrow_unix.go (Linux and macOS); its
//     behavior test is worker_borrow_unix_test.go.
//
// Installed fixture helpers carry no behavior of their own:
// activation_observation_installed_linux_test.go,
// hostile_snapshot_installed_linux_test.go.
package runtime
