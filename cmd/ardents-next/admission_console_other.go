//go:build !linux

package main

import (
	"errors"
	"os"
)

func admissionConsoleFile(*os.File) (*os.File, error) {
	return nil, errors.New("interactive admission requires Linux")
}
