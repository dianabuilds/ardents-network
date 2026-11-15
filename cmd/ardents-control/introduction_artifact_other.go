//go:build !linux

package main

import (
	"errors"
	"io"
)

func runIntroductionControl([]string, io.Writer) error {
	return errors.New("introduction artifact preparation requires Linux")
}
