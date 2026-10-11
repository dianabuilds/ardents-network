//go:build !linux && !windows

package durable

import "errors"

const platformSupported = false

type rootLease struct{}

func acquireLease(string) (rootLease, error) {
	return rootLease{}, errors.New("publication native lease unavailable")
}
func (*rootLease) release() error     { return nil }
func (*rootLease) check(string) error { return errors.New("publication native lease unavailable") }
func syncDirectory(string) error      { return errors.New("publication native directory flush unavailable") }
