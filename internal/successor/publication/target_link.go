package publication

import (
	"encoding/base64"
	"errors"
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
