//go:build !linux

package main

import (
	"errors"
	"os"
)

func openTextCommandIO() (*os.File, *os.File, error) {
	return nil, nil, errors.New("text UI requires the selected Linux host")
}
