package targetlink

import (
	"encoding/base64"
	"errors"
	"strings"
)

const (
	prefix          = "ardents-target:v1:"
	payloadSize     = 65
	algorithmOffset = 0
	networkOffset   = 1
	targetOffset    = networkOffset + 32
	targetAlgorithm = byte(1)
)

var (
	// ErrFormat reports text which is not the canonical Target Link v1 form.
	ErrFormat = errors.New("invalid Target Link v1 format")
	// ErrAlgorithm reports an unsupported Target algorithm identifier.
	ErrAlgorithm = errors.New("unsupported Target Link algorithm")
)

// Link is one fixed-width, explicit Target destination. Network and Target
// are intentionally opaque protocol values.
type Link struct {
	Network [32]byte
	Target  [32]byte
}

// Encode returns the only canonical Target Link v1 spelling for link.
func Encode(link Link) (string, error) {
	if empty(link.Network) || empty(link.Target) {
		return "", ErrFormat
	}
	payload := make([]byte, payloadSize)
	payload[algorithmOffset] = targetAlgorithm
	copy(payload[networkOffset:targetOffset], link.Network[:])
	copy(payload[targetOffset:], link.Target[:])
	return prefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

// Decode accepts only canonical, unpadded base64url Target Link v1 text.
func Decode(text string) (Link, error) {
	if !strings.HasPrefix(text, prefix) {
		return Link{}, ErrFormat
	}
	encoded := strings.TrimPrefix(text, prefix)
	if encoded == "" || strings.Contains(encoded, "=") {
		return Link{}, ErrFormat
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(payload) != payloadSize {
		return Link{}, ErrFormat
	}
	if payload[algorithmOffset] != targetAlgorithm {
		return Link{}, ErrAlgorithm
	}
	var link Link
	copy(link.Network[:], payload[networkOffset:targetOffset])
	copy(link.Target[:], payload[targetOffset:])
	canonical, err := Encode(link)
	if err != nil || canonical != text {
		return Link{}, ErrFormat
	}
	return link, nil
}

func empty(value [32]byte) bool {
	return value == [32]byte{}
}
