//go:build !linux

package instance

import "os"

// Private host-root custody is currently native Linux. Other hosts refuse
// before creating a key or filesystem state; public grammars remain portable.
const nativeSupported = false

type rootLease struct{}

func privateAccess(os.FileInfo) error           { return ErrUnavailable }
func prepareDirectory(string) error             { return ErrUnavailable }
func acquireLease(string) (rootLease, error)    { return rootLease{}, ErrUnavailable }
func (*rootLease) check(string) error           { return ErrUnavailable }
func (*rootLease) release() error               { return nil }
func syncDirectory(string) error                { return ErrUnavailable }
func readPrivate(string, int64) ([]byte, error) { return nil, ErrUnavailable }
