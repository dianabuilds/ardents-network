// Package outer serves one accepted Node Carrier above Route's authenticated
// handshake and bridge. It serializes writes across inner lanes, interrupts
// physical I/O on cancellation, and joins all handlers before returning.
// Serve returns the first physical close result to the receiving duty, which
// owns admission, roots, and the final accepted-connection result. Route owns
// framing and the bridge state.
package outer
