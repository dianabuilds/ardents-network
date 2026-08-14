//go:build !linux

package recoverysmoke

import "errors"

func platformCarrierSockets(string) ([]carrierObservation, error) {
	return nil, errors.New("carrier socket fault injection requires Linux")
}

func platformCarrierInterfaceForAddress(string) (string, int, error) {
	return "", 0, errors.New("carrier socket fault injection requires Linux")
}

func platformDeleteCarrierInterface(string) error {
	return errors.New("carrier socket fault injection requires Linux")
}

func platformCarrierSocketPresent([]byte) (bool, error) {
	return false, errors.New("carrier socket fault injection requires Linux")
}
