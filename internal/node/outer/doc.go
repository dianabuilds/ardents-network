// Package outer serves one accepted Node Carrier above Route's authenticated
// handshake and bridge. It serializes writes across inner lanes, interrupts
// physical I/O on cancellation, and joins all handlers before returning.
// The receiving duty owns admission, roots, and the final connection-close
// result; Route owns framing and the bridge state.
package outer
