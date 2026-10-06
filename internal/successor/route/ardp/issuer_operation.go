package ardp

import (
	"encoding/binary"
	"errors"
)

const IssuerBodySize = 16384

// EncodeIssuerRequest wraps the Admission-owned batch without interpreting it.
// Admission owns its exact length, proof and zero padding validation.
func EncodeIssuerRequest(nonce [32]byte, batch []byte) ([]byte, error) {
	if nonce == [32]byte{} || len(batch) == 0 || len(batch) > IssuerBodySize-33 {
		return nil, errors.New("route issuer request invalid")
	}
	body := make([]byte, IssuerBodySize)
	body[0] = 1
	copy(body[1:33], nonce[:])
	copy(body[33:], batch)
	return body, nil
}

// DecodeIssuerRequest borrows the padded Admission payload from body. It grants
// no permission and does not duplicate the Admission batch grammar.
func DecodeIssuerRequest(body []byte) ([32]byte, []byte, error) {
	var nonce [32]byte
	if len(body) != IssuerBodySize || body[0] != 1 {
		return nonce, nil, errors.New("route issuer operation invalid")
	}
	copy(nonce[:], body[1:33])
	if nonce == [32]byte{} {
		return nonce, nil, errors.New("route issuer nonce invalid")
	}
	return nonce, body[33:], nil
}

// EncodeIssuerResult returns the exact terminal envelope around the fixed
// Admission result payload. The caller owns the truthful outcome decision.
func EncodeIssuerResult(nonce [32]byte, status uint8, payload []byte) ([]byte, error) {
	if nonce == [32]byte{} || status > 4 || len(payload) != IssuerBodySize-37 {
		return nil, errors.New("route issuer result invalid")
	}
	body := make([]byte, IssuerBodySize)
	copy(body[:32], nonce[:])
	body[32] = status
	binary.BigEndian.PutUint32(body[33:37], uint32(len(payload)))
	copy(body[37:], payload)
	return body, nil
}

// DecodeIssuerResult checks this terminal's own nonce and fixed payload width.
// Admission and Stock separately verify outcome grammar and blinded signatures.
func DecodeIssuerResult(body []byte, nonce [32]byte) (uint8, []byte, error) {
	if len(body) != IssuerBodySize || nonce == [32]byte{} || body[32] > 4 || binary.BigEndian.Uint32(body[33:37]) != IssuerBodySize-37 {
		return 0, nil, errors.New("route issuer result invalid")
	}
	var actual [32]byte
	copy(actual[:], body[:32])
	if actual != nonce {
		return 0, nil, errors.New("route issuer result nonce differs")
	}
	return body[32], body[37:], nil
}
