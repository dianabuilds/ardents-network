// Package route implements the current closed Node Carrier and authenticated
// channel mechanics. The child ardp package owns generation-3 lane framing,
// HELLO and bootstrap bytes; terminal owns fixed operation bodies. The replay
// package owns durable receiving-duty token spends and Introduction slots.
// Closed Source lane state and deadlines live in closed_source_lane.go; its
// read/credit, write queue, and terminal retirement paths live in the
// corresponding closed_source_lane_read.go, _write.go, and _close.go files.
// Node's forwarding owner calls OpenClosedNodeCarrier for one exact
// State-selected TCP/TLS or QUIC-v2 attempt and retains peer selection and
// session lifetime. The production-dead Interactive User Route v2 Open/Attach
// owner, its private reachability exchange, and its relay and Introduction
// sender orchestration are absent. Shared Attachment evidence, Entry and
// Endpoint-transit admission, credential-relay grammar, native listeners, and
// reciprocal decoding remain only with their actual consumers. The old
// generation-2 Node-leg dial is absent. Route never chooses a fallback and has
// no H3 reader or peer runtime. The child capsule package owns the fixed
// Introduction submission and recipient HPKE codec; Route carries its opaque
// operation and does not interpret its private plaintext.
package route
