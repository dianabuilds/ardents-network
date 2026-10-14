// Package signing owns independently pinned, imported Ed25519 material and only
// validated issuer-profile signing. Copies retain one original lease, failure
// latch and joined Close. It exports no private key or arbitrary-byte signer.
//
// The existing identity root format and command names are compatibility surfaces;
// this supporting Admission mechanism grants no Node duty, runtime TLS, Network,
// Custody, quota or token-result authority. The pure issuerprofile parent never
// imports this private owner.
package signing
