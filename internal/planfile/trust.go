package planfile

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
)

// Authorities decodes one bounded authority key set indexed by key digest.
func Authorities(encoded []string, maximum int) (map[[32]byte]ed25519.PublicKey, error) {
	if len(encoded) == 0 || len(encoded) > maximum {
		return nil, errors.New("authority key count is invalid")
	}
	values := make(map[[32]byte]ed25519.PublicKey, len(encoded))
	for _, value := range encoded {
		public := make([]byte, ed25519.PublicKeySize)
		if err := FixedHex(value, public); err != nil {
			return nil, err
		}
		values[sha256.Sum256(public)] = ed25519.PublicKey(public)
	}
	return values, nil
}

// Digests decodes one bounded nonempty set of fixed 32-byte values.
func Digests(encoded []string, maximum int) ([][32]byte, error) {
	if len(encoded) == 0 || len(encoded) > maximum {
		return nil, errors.New("digest count is invalid")
	}
	values := make([][32]byte, len(encoded))
	for index, value := range encoded {
		if err := FixedHex(value, values[index][:]); err != nil {
			return nil, err
		}
	}
	return values, nil
}
