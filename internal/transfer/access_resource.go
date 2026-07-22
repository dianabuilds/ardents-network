package transfer

import (
	"errors"

	identitycontract "ardents/api/ardents/identity/v1"
)

func AccessResourceID(id string) (string, error) {
	if len(id) == 0 || len(id) > identitycontract.MaxCanonicalResourceIDBytes {
		return "", errors.New("transfer resource identifier is invalid")
	}
	for _, b := range []byte(id) {
		if b < 0x21 || b > 0x7e {
			return "", errors.New("transfer resource identifier is invalid")
		}
	}
	return id, nil
}
