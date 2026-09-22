// Package route retains the native Interactive Route v2 owner behind Open,
// Attach, and Close. That legacy owner currently has no non-test caller; its
// presence is not a maintained product path or evidence of successor-network
// readiness. The package also implements the current closed Node Carrier and
// wire mechanisms: Node's forwarding owner calls OpenClosedNodeCarrier for one
// exact State-selected TCP/TLS or QUIC-v2 attempt and retains peer selection and
// session lifetime. The old generation-2 Node-leg dial is absent; retained
// native listeners and reciprocal decoding remain with their actual consumers.
// The retained User Route owns only its sender-side Entry and relay exchanges;
// the retired Initiator receiving and direct OHTTP forwarding adapters are
// absent. Route never chooses a fallback and has no H3 reader or peer runtime.
package route
