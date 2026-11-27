// Package endpointstream owns the local Application byte-stream interface to
// the protected Endpoint. It carries explicit Target Links, bounded ordered
// bytes, directional EOF and one joined terminal outcome. Service Connection
// authentication and recovery belong to internal/domain/connection; local
// requests and sockets grant no authority to run a worker or choose a Route.
//
// Read the local operation in this order:
//   - request.go and target_link_input.go validate the request and local input;
//   - local_dial.go and client_stream.go own client setup and stream lifetime;
//   - server_linux.go and server_admission_linux.go own the Linux listener,
//     setup cancellation and retained refusal;
//   - transport_frames.go, server_stream_linux.go and terminal_outcome.go
//     own bounded frames, directional completion and exact terminal outcomes;
//   - stream_exchange.go and local_stream.go connect the interface to its
//     Application caller. Administration owns its separate local interface.
//
// Target Link files use no-follow, nonblocking opens on Linux and macOS;
// other platforms refuse before opening. The local server and its setup
// classifier require Linux. Portable stream entrypoints refuse other hosts
// before private input. Request encoding, the client, shared framing and typed
// outcomes remain portable. The corresponding behavior tests cover
// admission, private input refusal, cancellation, half-close and terminal bytes.
package endpointstream
