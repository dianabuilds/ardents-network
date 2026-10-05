// Package quic implements the selected single-stream QUIC transport. Shared
// transport owns requests, exact TLS authentication, handshake bounds and
// direct/Node classification. This adapter owns UDP sockets, arrival-bound
// handshake capacity, streams, native deadlines and physical interruption.
// Node and role principals remain distinct; only an authenticated role exposes
// a role exporter. It grants no Network, Admission, Route framing or durable
// authority. Composition joins returned borrowers before releasing resources.
package quic
