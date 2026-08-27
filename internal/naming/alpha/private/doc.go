// Package private owns the bounded alpha-only OHTTP resolution exchange. It
// keeps a Relay's opaque forwarding separate from a Gateway's decapsulated
// Alpha Service Link handling, retaining neither observation, and verifies a
// complete signed Alpha Name Corpus on every response. It never resolves
// canonical Namespace names or provides a plaintext fallback.
package private
