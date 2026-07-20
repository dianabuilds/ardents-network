package privacy

import (
	"encoding/binary"
	"fmt"
	"time"
)

const (
	envelopeVersion   = 1
	envelopeSuite     = 1
	headerSize        = 72
	maximumInnerSize  = 128 * 1024
	maximumOuterSize  = 132 * 1024
	maximumLifetime   = 30 * 24 * time.Hour
	maximumFutureSkew = 5 * time.Minute
)

var envelopeMagic = [4]byte{'A', 'R', 'D', 'P'}

type envelopeHeader struct {
	Version          byte
	Suite            byte
	Flags            uint16
	Generation       uint32
	IssuedAt         int64
	ExpiresAt        int64
	MessageID        [16]byte
	Nonce            [24]byte
	CiphertextLength uint32
}

func (h envelopeHeader) marshal() []byte {
	out := make([]byte, headerSize)
	copy(out[:4], envelopeMagic[:])
	out[4], out[5] = h.Version, h.Suite
	binary.BigEndian.PutUint16(out[6:8], h.Flags)
	binary.BigEndian.PutUint32(out[8:12], h.Generation)
	binary.BigEndian.PutUint64(out[12:20], uint64(h.IssuedAt))
	binary.BigEndian.PutUint64(out[20:28], uint64(h.ExpiresAt))
	copy(out[28:44], h.MessageID[:])
	copy(out[44:68], h.Nonce[:])
	binary.BigEndian.PutUint32(out[68:72], h.CiphertextLength)
	return out
}

func parseHeader(raw []byte) (envelopeHeader, error) {
	if len(raw) < headerSize {
		return envelopeHeader{}, envelopeError(CodeEnvelopeMalformed, "envelope header is truncated")
	}
	if string(raw[:4]) != string(envelopeMagic[:]) {
		return envelopeHeader{}, envelopeError(CodeEnvelopeMalformed, "envelope magic is invalid")
	}
	h := envelopeHeader{
		Version: raw[4], Suite: raw[5], Flags: binary.BigEndian.Uint16(raw[6:8]),
		Generation:       binary.BigEndian.Uint32(raw[8:12]),
		IssuedAt:         int64(binary.BigEndian.Uint64(raw[12:20])),
		ExpiresAt:        int64(binary.BigEndian.Uint64(raw[20:28])),
		CiphertextLength: binary.BigEndian.Uint32(raw[68:72]),
	}
	copy(h.MessageID[:], raw[28:44])
	copy(h.Nonce[:], raw[44:68])
	if h.Version != envelopeVersion {
		return envelopeHeader{}, envelopeError(CodeEnvelopeVersionUnsupported, "envelope version is unsupported")
	}
	if h.Suite != envelopeSuite {
		return envelopeHeader{}, envelopeError(CodeEnvelopeSuiteUnsupported, "envelope suite is unsupported")
	}
	if h.Flags != 0 {
		return envelopeHeader{}, envelopeError(CodeEnvelopeFlagsUnsupported, "envelope flags are unsupported")
	}
	if h.Generation == 0 || zeroBytes(h.MessageID[:]) || zeroBytes(h.Nonce[:]) {
		return envelopeHeader{}, envelopeError(CodeEnvelopeMalformed, "envelope identifiers are invalid")
	}
	if uint64(headerSize)+uint64(h.CiphertextLength) != uint64(len(raw)) {
		return envelopeHeader{}, envelopeError(CodeEnvelopeMalformed, "ciphertext length does not match envelope")
	}
	return h, nil
}

func validateTimes(h envelopeHeader, now time.Time) error {
	issued := time.Unix(h.IssuedAt, 0).UTC()
	expires := time.Unix(h.ExpiresAt, 0).UTC()
	if !issued.Before(expires) || expires.Sub(issued) > maximumLifetime {
		return envelopeError(CodeEnvelopeTimeInvalid, "envelope lifetime is invalid")
	}
	if issued.After(now.UTC().Add(maximumFutureSkew)) {
		return envelopeError(CodeEnvelopeTimeInvalid, "envelope issue time is too far in the future")
	}
	if !now.UTC().Before(expires) {
		return envelopeError(CodeEnvelopeExpired, "envelope is expired")
	}
	return nil
}

func associatedData(header []byte, pubsubTopic, contentTopic string) ([]byte, error) {
	if len(pubsubTopic) > 65535 || len(contentTopic) > 65535 {
		return nil, fmt.Errorf("Waku topic is too long")
	}
	out := append([]byte(nil), header...)
	for _, topic := range []string{pubsubTopic, contentTopic} {
		framed := make([]byte, 2)
		binary.BigEndian.PutUint16(framed, uint16(len(topic)))
		out = append(out, framed...)
		out = append(out, topic...)
	}
	return out, nil
}

func zeroBytes(raw []byte) bool {
	var combined byte
	for _, value := range raw {
		combined |= value
	}
	return combined == 0
}
