//go:build !linux

package main

import (
	"errors"
	"io"
)

func runForwardingControl([]string, io.Writer) error {
	return errors.New("forwarding artifact preparation requires Linux")
}
