//go:build !linux

package nodeidentity

import "os"

func platform() error                        { return ErrUnsupported }
func privateFile(os.FileInfo, bool) bool     { return false }
func acquireLock(*os.Root) (*os.File, error) { return nil, ErrUnsupported }
func releaseLock(*os.File) error             { return nil }
