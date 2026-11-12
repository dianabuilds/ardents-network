//go:build !linux

package main

import (
	"errors"
	"io"
)

func runJoinControl([]string, io.Writer) error {
	return errors.New("JOIN artifact preparation requires Linux")
}
