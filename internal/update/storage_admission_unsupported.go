//go:build !linux && !windows

package update

import "errors"

func observeOwnedStorage(string) (resourceObservation, error) {
	return resourceObservation{}, errors.New("unsupported update storage platform")
}
