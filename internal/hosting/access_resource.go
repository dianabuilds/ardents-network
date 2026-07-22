package hosting

import (
	"errors"

	identitycontract "ardents/api/ardents/identity/v1"
)

// ServiceAccessResourceID validates the exact ServiceID used at an access
// boundary. Service IDs remain service identifiers; they are never Principals.
func ServiceAccessResourceID(id string) (string, error) {
	if len(id) == 0 || len(id) > identitycontract.MaxCanonicalResourceIDBytes {
		return "", errors.New("service resource identifier is invalid")
	}
	for _, b := range []byte(id) {
		if b < 0x21 || b > 0x7e {
			return "", errors.New("service resource identifier is invalid")
		}
	}
	return id, nil
}
