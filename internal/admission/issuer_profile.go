package admission

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

const (
	closedIssuerProfileMagic = "ARDCIP01"

	// MaximumClosedIssuerKeys bounds one finite Node-signed public inventory.
	MaximumClosedIssuerKeys = 18
)

// closedIssuerDomain prefixes the exact transcript the Node identity key signs.
var closedIssuerDomain = []byte("ardents-closed-issuer-keys-v1\x00")

// ClosedIssuerKey is one public class/window RSA-PSS configuration from an
// immutable closed issuer root.
type ClosedIssuerKey struct {
	WindowStart time.Time
	Class       byte
	SPKI        []byte
	KeyID       [32]byte
}

// ClosedIssuerProfile is the Node-signed public inventory that State uses as
// exact input when preparing the closed profile. It contains no private key,
// permission, holder, or admission-authority material.
type ClosedIssuerProfile struct {
	NetworkID, NodeID   [32]byte
	NotBefore, NotAfter time.Time
	Keys                []ClosedIssuerKey
	Signature           [ed25519.SignatureSize]byte
}

// ClosedIssuerKeyCount returns the exact canonical key inventory for one
// finite hourly interval, or zero when the interval facts are invalid.
func ClosedIssuerKeyCount(notBefore, notAfter time.Time) int {
	if notBefore.IsZero() || notAfter.IsZero() || notBefore.Truncate(time.Hour) != notBefore ||
		notAfter.Truncate(time.Hour) != notAfter || !notAfter.After(notBefore) || notAfter.After(notBefore.Add(6*time.Hour)) {
		return 0
	}
	return int(notAfter.Sub(notBefore)/time.Hour) * 3
}

// EncodeClosedIssuerProfile returns the canonical Node-signed public key
// inventory. It is the only writer of the ARDCIP01 grammar; the bytes are
// identical to the historical route/credential encoder (F-28/F-30 seam).
func EncodeClosedIssuerProfile(profile ClosedIssuerProfile, signer ed25519.PrivateKey) ([]byte, error) {
	if len(signer) != ed25519.PrivateKeySize || len(profile.Keys) == 0 || len(profile.Keys) > MaximumClosedIssuerKeys {
		return nil, errors.New("closed issuer public profile is invalid")
	}
	raw := make([]byte, 0, 128+len(profile.Keys)*360)
	raw = append(raw, closedIssuerProfileMagic...)
	for _, item := range [][32]byte{profile.NetworkID, profile.NodeID} {
		raw = append(raw, item[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, uint64(profile.NotBefore.Unix()))
	raw = binary.BigEndian.AppendUint64(raw, uint64(profile.NotAfter.Unix()))
	raw = binary.BigEndian.AppendUint16(raw, uint16(len(profile.Keys)))
	for index, key := range profile.Keys {
		if key.WindowStart.Truncate(time.Hour) != key.WindowStart || key.WindowStart.Before(profile.NotBefore) || !key.WindowStart.Before(profile.NotAfter) ||
			key.Class < 1 || key.Class > 3 || len(key.SPKI) != 346 || !state.ValidateClosedTokenSPKI(key.SPKI) || key.KeyID != sha256.Sum256(key.SPKI) ||
			index > 0 && (profile.Keys[index-1].WindowStart.After(key.WindowStart) ||
				profile.Keys[index-1].WindowStart.Equal(key.WindowStart) && profile.Keys[index-1].Class >= key.Class) {
			return nil, errors.New("closed issuer public profile keys are invalid")
		}
		raw = binary.BigEndian.AppendUint64(raw, uint64(key.WindowStart.Unix()))
		raw = append(raw, key.Class)
		raw = binary.BigEndian.AppendUint16(raw, uint16(len(key.SPKI)))
		raw = append(raw, key.SPKI...)
	}
	return append(raw, ed25519.Sign(signer, closedIssuerTranscript(raw))...), nil
}

func closedIssuerTranscript(body []byte) []byte {
	transcript := make([]byte, 0, len(closedIssuerDomain)+len(body))
	transcript = append(transcript, closedIssuerDomain...)
	return append(transcript, body...)
}

// DecodeClosedIssuerProfile verifies one exact Node-signed public key profile.
func DecodeClosedIssuerProfile(raw []byte, nodePublic ed25519.PublicKey) (ClosedIssuerProfile, error) {
	if len(nodePublic) != ed25519.PublicKeySize || len(raw) < len(closedIssuerProfileMagic)+32+32+8+8+2+ed25519.SignatureSize ||
		!ed25519.Verify(nodePublic, closedIssuerTranscript(raw[:len(raw)-ed25519.SignatureSize]), raw[len(raw)-ed25519.SignatureSize:]) {
		return ClosedIssuerProfile{}, errors.New("closed issuer public profile signature is invalid")
	}
	body := raw[:len(raw)-ed25519.SignatureSize]
	if string(body[:8]) != closedIssuerProfileMagic {
		return ClosedIssuerProfile{}, errors.New("closed issuer public profile magic is invalid")
	}
	profile := ClosedIssuerProfile{}
	offset := 8
	for _, item := range []*[32]byte{&profile.NetworkID, &profile.NodeID} {
		copy(item[:], body[offset:offset+32])
		offset += 32
	}
	profile.NotBefore = time.Unix(int64(binary.BigEndian.Uint64(body[offset:offset+8])), 0).UTC()
	offset += 8
	profile.NotAfter = time.Unix(int64(binary.BigEndian.Uint64(body[offset:offset+8])), 0).UTC()
	offset += 8
	count := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if profile.NetworkID == [32]byte{} || profile.NodeID == [32]byte{} || count == 0 || count > MaximumClosedIssuerKeys || count != ClosedIssuerKeyCount(profile.NotBefore, profile.NotAfter) {
		return ClosedIssuerProfile{}, errors.New("closed issuer public profile facts are invalid")
	}
	for index := 0; index < count; index++ {
		if offset+11 > len(body) {
			return ClosedIssuerProfile{}, errors.New("closed issuer public profile is truncated")
		}
		key := ClosedIssuerKey{WindowStart: time.Unix(int64(binary.BigEndian.Uint64(body[offset:offset+8])), 0).UTC()}
		offset += 8
		key.Class = body[offset]
		offset++
		length := int(binary.BigEndian.Uint16(body[offset : offset+2]))
		offset += 2
		if length != 346 || offset+length > len(body) {
			return ClosedIssuerProfile{}, errors.New("closed issuer public profile key is invalid")
		}
		key.SPKI = append([]byte(nil), body[offset:offset+length]...)
		offset += length
		key.KeyID = sha256.Sum256(key.SPKI)
		if key.WindowStart.Truncate(time.Hour) != key.WindowStart || key.WindowStart.Before(profile.NotBefore) || !key.WindowStart.Before(profile.NotAfter) ||
			key.Class < 1 || key.Class > 3 || !state.ValidateClosedTokenSPKI(key.SPKI) ||
			len(profile.Keys) > 0 && (profile.Keys[len(profile.Keys)-1].WindowStart.After(key.WindowStart) ||
				profile.Keys[len(profile.Keys)-1].WindowStart.Equal(key.WindowStart) && profile.Keys[len(profile.Keys)-1].Class >= key.Class) {
			return ClosedIssuerProfile{}, errors.New("closed issuer public profile keys are not canonical")
		}
		profile.Keys = append(profile.Keys, key)
	}
	if offset != len(body) {
		return ClosedIssuerProfile{}, errors.New("closed issuer public profile framing is invalid")
	}
	copy(profile.Signature[:], raw[len(body):])
	return profile, nil
}
