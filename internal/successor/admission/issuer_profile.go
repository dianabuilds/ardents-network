package admission

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"time"
)

const MaximumIssuerProfile = 6580
const issuerProfileDomain = "ardents-closed-issuer-keys-v1\x00"

// IssuerProfileBinding consists of independently selected offline public pins.
type IssuerProfileBinding struct {
	Network, Issuer, Signer [32]byte
	Start, End              time.Time
}

func (b IssuerProfileBinding) ledger(keys []TokenKey) LedgerBinding {
	return LedgerBinding{Network: b.Network, Issuer: b.Issuer, Authority: [32]byte{1}, Profile: [32]byte{1}, Duty: 1, Start: b.Start, End: b.End, Keys: keys}
}
func (b IssuerProfileBinding) matches(other IssuerProfileBinding) bool {
	return b.Network == other.Network && b.Issuer == other.Issuer && b.Signer == other.Signer && b.Start.Equal(other.Start) && b.End.Equal(other.End)
}

type issuerProfileEvidence struct {
	binding IssuerProfileBinding
	body    []byte
	keys    []TokenKey
}

// IssuerProfileRequest permits only one validated, domain-specific signature.
type IssuerProfileRequest struct{ evidence *issuerProfileEvidence }

// VerifiedIssuerProfile proves only signature and exact pinned inventory facts.
type VerifiedIssuerProfile struct{ evidence *issuerProfileEvidence }

func PrepareIssuerProfile(b IssuerProfileBinding, keys []TokenKey) (IssuerProfileRequest, error) {
	if b.Signer == ([32]byte{}) || !b.ledger(keys).valid() {
		return IssuerProfileRequest{}, ErrInvalid
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
	return IssuerProfileRequest{&issuerProfileEvidence{binding: b, body: body, keys: cloneBinding(b.ledger(keys)).Keys}}, nil
}

// Snapshot returns a protected binding and exact domain-prefixed transcript.
func (r IssuerProfileRequest) Snapshot() (IssuerProfileBinding, []byte, bool) {
	if r.evidence == nil {
		return IssuerProfileBinding{}, nil, false
	}
	return r.evidence.binding, append([]byte(issuerProfileDomain), r.evidence.body...), true
}

// Complete checks a Node signature before emitting the canonical profile.
func (r IssuerProfileRequest) Complete(signature []byte) ([]byte, error) {
	b, transcript, ok := r.Snapshot()
	if !ok || !ed25519.Verify(b.Signer[:], transcript, signature) {
		return nil, ErrInvalid
	}
	return append(append([]byte(nil), r.evidence.body...), signature...), nil
}
func VerifyIssuerProfile(raw []byte, expected IssuerProfileBinding) (VerifiedIssuerProfile, error) {
	if len(raw) < 154 || len(raw) > MaximumIssuerProfile || expected.Signer == ([32]byte{}) {
		return VerifiedIssuerProfile{}, ErrInvalid
	}
	body := raw[:len(raw)-64]
	if string(body[:8]) != "ARDCIP01" || !ed25519.Verify(expected.Signer[:], append([]byte(issuerProfileDomain), body...), raw[len(body):]) {
		return VerifiedIssuerProfile{}, ErrInvalid
	}
	var b IssuerProfileBinding
	copy(b.Network[:], body[8:40])
	copy(b.Issuer[:], body[40:72])
	b.Signer = expected.Signer
	start, end := binary.BigEndian.Uint64(body[72:80]), binary.BigEndian.Uint64(body[80:88])
	if start > uint64(1<<63-1) || end > uint64(1<<63-1) {
		return VerifiedIssuerProfile{}, ErrInvalid
	}
	b.Start = time.Unix(int64(start), 0).UTC()
	b.End = time.Unix(int64(end), 0).UTC()
	if !b.matches(expected) {
		return VerifiedIssuerProfile{}, ErrInvalid
	}
	count := int(binary.BigEndian.Uint16(body[88:90]))
	if count < 3 || count > 18 || len(body) != 90+count*357 {
		return VerifiedIssuerProfile{}, ErrInvalid
	}
	keys := make([]TokenKey, 0, count)
	for i := 0; i < count; i++ {
		off := 90 + i*357
		if binary.BigEndian.Uint16(body[off+9:off+11]) != 346 {
			return VerifiedIssuerProfile{}, ErrInvalid
		}
		keys = append(keys, TokenKey{Window: binary.BigEndian.Uint64(body[off : off+8]), Class: body[off+8], SPKI: append([]byte(nil), body[off+11:off+357]...)})
	}
	request, err := PrepareIssuerProfile(b, keys)
	if err != nil || !bytes.Equal(request.evidence.body, body) {
		return VerifiedIssuerProfile{}, ErrInvalid
	}
	return VerifiedIssuerProfile(request), nil
}
func (v VerifiedIssuerProfile) Snapshot() (IssuerProfileBinding, []TokenKey, bool) {
	if v.evidence == nil {
		return IssuerProfileBinding{}, nil, false
	}
	return v.evidence.binding, cloneBinding(v.evidence.binding.ledger(v.evidence.keys)).Keys, true
}

// PrepareLedgerBinding fills only keys; State profile, authority and duty persist.
func PrepareLedgerBinding(base LedgerBinding, v VerifiedIssuerProfile) (LedgerBinding, error) {
	b, keys, ok := v.Snapshot()
	if !ok || len(base.Keys) != 0 || base.Network != b.Network || base.Issuer != b.Issuer || !base.Start.Equal(b.Start) || !base.End.Equal(b.End) {
		return LedgerBinding{}, ErrInvalid
	}
	base.Keys = keys
	if !base.valid() {
		return LedgerBinding{}, ErrInvalid
	}
	return base, nil
}
