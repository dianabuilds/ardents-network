// Package route implements the current closed Node Carrier and wire mechanisms.
// Node's forwarding owner calls OpenClosedNodeCarrier for one exact
// State-selected TCP/TLS or QUIC-v2 attempt and retains peer selection and
// session lifetime. The production-dead Interactive User Route v2 Open/Attach
// owner, its private reachability exchange, and its relay and Introduction
// sender orchestration are absent. Shared Attachment evidence, Entry and
// Endpoint-transit admission, credential-relay grammar, and native listeners
// remain only with their actual consumers. The reciprocal codec is test-only
// compatibility evidence. The old
// generation-2 Node-leg dial and v1 Node Carrier listener are absent. Route
// never chooses a fallback and has no H3 reader or peer runtime.
package route
