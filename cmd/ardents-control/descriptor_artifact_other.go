//go:build !linux

package main

import (
	"errors"
	"io"
)

func runDescriptorControl([]string, io.Writer) error {
	return errors.New("Descriptor artifact preparation requires Linux")
}
