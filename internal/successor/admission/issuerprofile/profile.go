package issuerprofile

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"time"
)

const MaximumSize = 6580
const issuerProfileDomain = "ardents-closed-issuer-keys-v1\x00"

// Binding consists of independently selected offline public pins.
type Binding struct {
	Network, Issuer, Signer [32]byte
	Start, End              time.Time
}

func (b Binding) matches(other Binding) bool {
	return b.Network == other.Network && b.Issuer == other.Issuer && b.Signer == other.Signer && b.Start.Equal(other.Start) && b.End.Equal(other.End)
}

type issuerProfileEvidence struct {
	binding Binding
	body    []byte
	keys    []Key
}

// Request permits only one validated, domain-specific signature.
type Request struct{ evidence *issuerProfileEvidence }

// Verified proves only signature and exact pinned inventory facts.
// Its representation is deliberately different from Request: Go conversion
// must not promote unsigned preparation into signature-checked evidence.
type Verified struct{ signatureChecked *issuerProfileEvidence }

func Prepare(b Binding, keys []Key) (Request, error) {
	if b.Network == ([32]byte{}) || b.Issuer == ([32]byte{}) || b.Signer == ([32]byte{}) || ValidateCohorts(b.Start, b.End, keys) != nil {
		return Request{}, ErrInvalid
	}
	body := append([]byte("ARDCIP01"), b.Network[:]...)
	body = append(body, b.Issuer[:]...)
	body = binary.BigEndian.AppendUint64(body, uint64(b.Start.Unix()))
	body = binary.BigEndian.AppendUint64(body, uint64(b.End.Unix()))
	body = binary.BigEndian.AppendUint16(body, uint16(len(keys)))
	for _, k := range keys {
		body = binary.BigEndian.AppendUint64(body, k.Window)
		body = append(body, k.Class)
		body = binary.BigEndian.AppendUint16(body, uint16(len(k.SPKI)))
		body = append(body, k.SPKI...)
	}
	return Request{&issuerProfileEvidence{binding: b, body: body, keys: cloneKeys(keys)}}, nil
}

// Snapshot returns a protected binding and exact domain-prefixed transcript.
func (r Request) Snapshot() (Binding, []byte, bool) {
	if r.evidence == nil {
		return Binding{}, nil, false
	}
	return r.evidence.binding, append([]byte(issuerProfileDomain), r.evidence.body...), true
}

// Complete checks a Node signature before emitting the canonical profile.
func (r Request) Complete(signature []byte) ([]byte, error) {
	b, transcript, ok := r.Snapshot()
	if !ok || !ed25519.Verify(b.Signer[:], transcript, signature) {
		return nil, ErrInvalid
	}
	return append(append([]byte(nil), r.evidence.body...), signature...), nil
}
func Verify(raw []byte, expected Binding) (Verified, error) {
	if len(raw) < 154 || len(raw) > MaximumSize || expected.Signer == ([32]byte{}) {
		return Verified{}, ErrInvalid
	}
	body := raw[:len(raw)-64]
	if string(body[:8]) != "ARDCIP01" || !ed25519.Verify(expected.Signer[:], append([]byte(issuerProfileDomain), body...), raw[len(body):]) {
		return Verified{}, ErrInvalid
	}
	var b Binding
	copy(b.Network[:], body[8:40])
	copy(b.Issuer[:], body[40:72])
	b.Signer = expected.Signer
	start, end := binary.BigEndian.Uint64(body[72:80]), binary.BigEndian.Uint64(body[80:88])
	if start > uint64(1<<63-1) || end > uint64(1<<63-1) {
		return Verified{}, ErrInvalid
	}
	b.Start = time.Unix(int64(start), 0).UTC()
	b.End = time.Unix(int64(end), 0).UTC()
	if !b.matches(expected) {
		return Verified{}, ErrInvalid
	}
	count := int(binary.BigEndian.Uint16(body[88:90]))
	if count < 3 || count > 18 || len(body) != 90+count*357 {
		return Verified{}, ErrInvalid
	}
	keys := make([]Key, 0, count)
	for i := 0; i < count; i++ {
		off := 90 + i*357
		if binary.BigEndian.Uint16(body[off+9:off+11]) != 346 {
			return Verified{}, ErrInvalid
		}
		keys = append(keys, Key{Window: binary.BigEndian.Uint64(body[off : off+8]), Class: body[off+8], SPKI: append([]byte(nil), body[off+11:off+357]...)})
	}
	request, err := Prepare(b, keys)
	if err != nil || !bytes.Equal(request.evidence.body, body) {
		return Verified{}, ErrInvalid
	}
	return Verified{signatureChecked: request.evidence}, nil
}
func (v Verified) Snapshot() (Binding, []Key, bool) {
	if v.signatureChecked == nil {
		return Binding{}, nil, false
	}
	return v.signatureChecked.binding, cloneKeys(v.signatureChecked.keys), true
}
