// Package source implements the private, finite Direct-Origin Source
// transport used to acquire and redistribute authenticated Network State. It
// owns bounded network-scoped request, response, and private bundle framing with pinned mutual
// TLS, but does not make the transport a public wire protocol or decide whether
// state is acceptable. Its pure local serving-declaration codec is shared by
// command composition and Installation's future-input matching; decoding grants
// neither native preflight admission nor runtime authority.
package source
