// Package tls implements the selected TCP/TLS transport: direct role dial,
// mutual Node Carrier, shared listener and inner role TLS over selected lanes.
// Shared transport fixes authentication and input bounds. This adapter owns
// sockets, finite handshake capacity, deadlines and physical Node interruption;
// it grants no Network, Admission, path-selection or durable-root authority.
// OpenRole/AcceptRole callers retain their supplied raw connection on refusal;
// dial operations close their own failed socket. Returned connections transfer
// to composition, which joins borrowers before releasing domain resources.
// Inner role TLS consumes ciphertext through one bounded 16 KiB input buffer;
// small TLS header reads do not each consume a new forwarding-lane read. This
// consumer buffer performs no background read or timed accumulation and retains
// the raw owner's deadlines, interruption and framing-retirement traversal.
package tls
