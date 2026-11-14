//go:build !linux

package main

import (
	"errors"
	"io"
)

func runDescriptorControl([]string, io.Writer) error {
	return errors.New("descriptor artifact preparation requires Linux")
}
