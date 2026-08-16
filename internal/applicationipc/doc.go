// Package applicationipc preserves the raw Application byte stream and owns
// its bounded Result frame on either the legacy stream tail or an optional
// derived owner-only local control channel.
package applicationipc
