// Package carrier owns the physical closed transport beneath Route and Node:
// the exact State-authorized v3 TCP/TLS and QUIC-v2 Node-Carrier dial, the
// role TLS server and client handshakes with exact-peer authentication, the
// shared post-TLS classified Carrier listeners, the leased closed Carrier
// pool, the retired-transport close wrappers, and the literal-address rule.
//
// A Carrier is one transport-neutral byte lane; transport addresses, QUIC
// state, fallback, and migration stay private. The package selects no peer
// and composes no plan: peer selection, plan composition, admission, and
// session lifetime stay with the callers in Route, Node, and Credential. The
// retired v2 Route profile identity stays in Route beside its typed refusal.
package carrier
