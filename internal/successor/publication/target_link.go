package publication

import (
	"encoding/base64"
	"errors"
	"strings"
)

// TargetLink encodes the accepted fixed-width v3 public destination grammar.
// A public Link never attests accepting readiness; the live owner decides when
// to expose it. No old runtime codec or mutable owner is imported.
func TargetLink(network, target [32]byte) (string, error) {
	if network == [32]byte{} || target == [32]byte{} {
		return "", errors.New("Target Link binding absent")
	}
	raw := append([]byte{1}, network[:]...)
	raw = append(raw, target[:]...)
	return "ardents-target:v3:" + base64.RawURLEncoding.EncodeToString(raw), nil
}

// ParseTargetLink accepts only the canonical closed v3 destination. It grants
// no proof, readiness or authority and supplies no alternate-Target fallback.
func ParseTargetLink(link string) (network, target [32]byte, err error) {
	const prefix = "ardents-target:v3:"
	if !strings.HasPrefix(link, prefix) || len(link) != len(prefix)+87 {
		return network, target, errors.New("Target Link format is invalid")
	}
	encoded := link[len(prefix):]
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) != 65 || raw[0] != 1 || base64.RawURLEncoding.EncodeToString(raw) != encoded {
		return network, target, errors.New("Target Link format is invalid")
	}
	copy(network[:], raw[1:33])
	copy(target[:], raw[33:])
	if network == [32]byte{} || target == [32]byte{} {
		return [32]byte{}, [32]byte{}, errors.New("Target Link binding is absent")
	}
	return network, target, nil
}
