// Package source implements the private, finite Direct-Origin Source
// transport used to acquire and redistribute authenticated Network State. It
// owns bounded network-scoped request, response, and private bundle framing with pinned mutual
// TLS, but does not make the transport a public wire protocol or decide whether
// state is acceptable.
package source
