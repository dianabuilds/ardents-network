// Package principal owns principal derivation and canonical identity encoding.
// It does not own credential storage or authorization policy.
package principal

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"strings"
)

const (
	principalPrefix     = "p1_"
	devicePrefix        = "d1_"
	encodedDigestLength = 52
)

var (
	principalDomain = []byte("ardents:principal:v1\x00")
	deviceDomain    = []byte("ardents:device:v1\x00")
	base32Lower     = base32.StdEncoding.WithPadding(base32.NoPadding)
	errInvalidID    = errors.New("identity identifier is invalid")
)

type ID struct{ digest [sha256.Size]byte }
type DeviceID struct{ digest [sha256.Size]byte }

func FromEd25519PublicKey(public ed25519.PublicKey) (ID, error) {
	digest, err := derive(public, principalDomain)
	if err != nil {
		return ID{}, err
	}
	return ID{digest: digest}, nil
}

func DeviceFromEd25519PublicKey(public ed25519.PublicKey) (DeviceID, error) {
	digest, err := derive(public, deviceDomain)
	if err != nil {
		return DeviceID{}, err
	}
	return DeviceID{digest: digest}, nil
}

func Parse(value string) (ID, error) {
	digest, err := parse(value, principalPrefix)
	if err != nil {
		return ID{}, err
	}
	return ID{digest: digest}, nil
}
func ParseDeviceID(value string) (DeviceID, error) {
	digest, err := parse(value, devicePrefix)
	if err != nil {
		return DeviceID{}, err
	}
	return DeviceID{digest: digest}, nil
}
func (id ID) String() string {
	if id.digest == ([sha256.Size]byte{}) {
		return ""
	}
	return principalPrefix + strings.ToLower(base32Lower.EncodeToString(id.digest[:]))
}
func (id DeviceID) String() string {
	if id.digest == ([sha256.Size]byte{}) {
		return ""
	}
	return devicePrefix + strings.ToLower(base32Lower.EncodeToString(id.digest[:]))
}
func (id ID) Equal(other ID) bool             { return id.digest == other.digest }
func (id DeviceID) Equal(other DeviceID) bool { return id.digest == other.digest }
func (id ID) MarshalText() ([]byte, error) {
	if id.String() == "" {
		return nil, errInvalidID
	}
	return []byte(id.String()), nil
}
func (id DeviceID) MarshalText() ([]byte, error) {
	if id.String() == "" {
		return nil, errInvalidID
	}
	return []byte(id.String()), nil
}
func (id *ID) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}
func (id *DeviceID) UnmarshalText(text []byte) error {
	parsed, err := ParseDeviceID(string(text))
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

func derive(public ed25519.PublicKey, domain []byte) ([sha256.Size]byte, error) {
	if len(public) != ed25519.PublicKeySize {
		return [sha256.Size]byte{}, errInvalidID
	}
	payload := make([]byte, 0, len(domain)+1+len(public))
	payload = append(payload, domain...)
	payload = append(payload, 1)
	payload = append(payload, public...)
	return sha256.Sum256(payload), nil
}

func parse(value, prefix string) ([sha256.Size]byte, error) {
	if len(value) != len(prefix)+encodedDigestLength || !strings.HasPrefix(value, prefix) || value != strings.ToLower(value) {
		return [sha256.Size]byte{}, errInvalidID
	}
	raw, err := base32Lower.DecodeString(strings.ToUpper(value[len(prefix):]))
	if err != nil || len(raw) != sha256.Size {
		return [sha256.Size]byte{}, errInvalidID
	}
	if strings.ToLower(base32Lower.EncodeToString(raw)) != value[len(prefix):] {
		return [sha256.Size]byte{}, errInvalidID
	}
	var digest [sha256.Size]byte
	copy(digest[:], raw)
	if digest == ([sha256.Size]byte{}) {
		return [sha256.Size]byte{}, errInvalidID
	}
	return digest, nil
}

func FromPublicKey(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("record public key is invalid")
	}
	if len(raw) != ed25519.PublicKeySize {
		return "", errors.New("record public key length is invalid")
	}
	id, err := FromEd25519PublicKey(ed25519.PublicKey(raw))
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
