package records

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"

	identitycontract "ardents/api/ardents/identity/v1"
)

const discoveryResourceDomain = "ardents:discovery-record-resource:v1\x00"

// AccessResourceID is the target-module-owned canonical ID for the exact
// (kind, subject) lookup tuple used by Operator access control.
func AccessResourceID(kind, subject string) (string, error) {
	if !validResourcePart(kind) || !validResourcePart(subject) {
		return "", errors.New("discovery resource selector is invalid")
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(discoveryResourceDomain))
	for _, value := range []string{kind, subject} {
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(value)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(value))
	}
	return "drr1_" + base64.RawURLEncoding.EncodeToString(hash.Sum(nil)), nil
}

func ValidAccessIdentifier(value string) bool { return validResourcePart(value) }

func validResourcePart(value string) bool {
	if len(value) == 0 || len(value) > identitycontract.MaxCanonicalResourceIDBytes {
		return false
	}
	for _, b := range []byte(value) {
		if b < 0x21 || b > 0x7e {
			return false
		}
	}
	return true
}
