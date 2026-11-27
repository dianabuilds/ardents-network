// Package endpointstream owns the shared local Application stream contract:
// bounded requests, byte frames and terminal outcomes.
// Requests and sockets grant no worker or Route authority. Authenticated
// Service Connection and its recovery belong to internal/domain/connection.
//
// A local operation follows these owners:
//   - request.go and targetlink validate the request and local input;
//   - client owns dial, cancellation handoff, frame I/O and joined closure;
//   - server owns the Linux listener, admission, refusal and server cleanup;
//   - transport_frames.go and terminal_outcome.go define their shared bytes
//     and results; application/console owns local presentation descriptors.
//
// Target Link files use no-follow, nonblocking opens on Linux and macOS;
// other platforms refuse before opening. The server and its request reader
// require Linux; the client refuses unsupported hosts before private input.
// Portable request encoding, client framing and typed outcomes remain shared.
package endpointstream
