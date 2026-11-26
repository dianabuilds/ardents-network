// Package connection owns the local Connection contract for the protected
// Endpoint. AAI3 selects the document profile and AAI4 selects the bidirectional
// byte-stream profile. Both use explicit Target Links, bounded ordered streams,
// directional EOF and one joined terminal outcome. Name remains reserved.
//
// The retired AAI2 magic is recognized for refusal before Interface.Open.
// Changes to this owner require admission, cancellation, half-close and
// terminal-result regression evidence.
//
// The pure typed refusal constructor Refuse (server_refusal.go) is portable;
// the refusal setupCleanupError classifier and the server admission stay with
// the protected Linux Endpoint (#885).
//
// Responsibility clusters and their tests:
//   - stream interface and limits: stream_interface.go
//   - terminal outcomes and setup refusals: terminal_outcome.go
//   - request grammar: request.go, request_decode_linux.go
//     (request_test.go; decoding is Linux)
//   - Target Link input reading: target_link_input.go, private_link.go,
//     private_link_unix.go, private_link_other.go
//     (stream_private_link_linux_test.go; linux, exercised through the real
//     CLI binary built by command_binary_linux_test.go)
//   - client setup and cancellation handoff: local_dial.go
//     (client_setup_cancellation_test.go)
//   - client stream lifecycle and response reception: client_stream.go,
//     client_receive.go (client_close_test.go); client_unix_peer_test.go
//     provides a wire peer for both client test groups
//   - shared frame encoding and decoding: transport_frames.go
//   - server transport and lifecycle: transport.go,
//     server_admission.go (transport_test.go, server_lifecycle_test.go,
//     server_cleanup_test.go, server_setup_cleanup_test.go,
//     transport_binary_conformance_test.go)
//   - typed refusal: server_refusal.go (portable Refuse),
//     server_refusal_linux.go (setupCleanupError classifier; linux)
//     (server_refusal_test.go, server_refusal_linux_test.go)
//   - application stream exchange and byte-stream host: stream_exchange.go,
//     stream_host_linux.go, stream_host_other.go (stream_exchange_test.go,
//     response_eof_linux_test.go; linux)
//   - local console access: local_console_linux.go, local_console_other.go,
//     local_stream.go (local_console_linux_test.go; linux)
package connection
