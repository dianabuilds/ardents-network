//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package main

import (
	"errors"
	"os"
)

func openInputFile(string) (*os.File, error) {
	return nil, errors.New("bounded input opening unavailable on this platform")
}
