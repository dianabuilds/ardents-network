//go:build !linux

package main

import (
	"errors"
	"io"
)

func runSourceControl([]string, io.Writer) error {
	return errors.New("source control requires Linux")
}
