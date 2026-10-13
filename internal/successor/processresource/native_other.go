//go:build !linux

package processresource

import "errors"

type nativePin struct{}

func openNative() (*nativePin, error) {
	return nil, errors.New("native process protection requires Linux")
}
func (*nativePin) observe() (Observation, error) {
	return Observation{}, errors.New("native process observation requires Linux")
}
func (*nativePin) close() error { return nil }
