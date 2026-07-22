package discovery

import (
	"errors"

	discoveryrecord "ardents/internal/discovery/records"
)

func ServiceAccessResourceID(service string) (string, error) {
	if !discoveryrecord.ValidAccessIdentifier(service) {
		return "", errors.New("service resource identifier is invalid")
	}
	return service, nil
}
