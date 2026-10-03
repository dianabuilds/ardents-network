//go:build !linux

package quota

import "os"

func admissionPlatform() error                        { return ErrUnsupported }
func privateAdmissionFile(os.FileInfo, bool) bool     { return false }
func acquireAdmissionLock(*os.Root) (*os.File, error) { return nil, ErrUnsupported }
func releaseAdmissionLock(*os.File) error             { return nil }
