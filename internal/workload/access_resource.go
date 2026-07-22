package workload

import (
	"errors"

	identitycontract "ardents/api/ardents/identity/v1"
)

// AccessResourceID validates the exact WorkloadID used at an access boundary.
// Workload IDs remain workload identifiers; they are never Principal IDs.
func AccessResourceID(id string) (string, error) {
	if !validAccessIdentifier(id) {
		return "", errors.New("workload resource identifier is invalid")
	}
	return id, nil
}

func validAccessIdentifier(value string) bool {
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
