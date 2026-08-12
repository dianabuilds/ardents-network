// Package source implements the private, finite Direct-Origin Source
// transport used to acquire and redistribute authenticated Network State. It
// provides bounded canonical framing and pinned mutual TLS, but does not make
// the transport a public wire protocol or decide whether state is acceptable.
package source
