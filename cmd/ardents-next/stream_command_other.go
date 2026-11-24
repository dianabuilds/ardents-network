//go:build !linux

package main

import (
	"context"
	"errors"
)

var errApplicationStreamHost = errors.New("application stream requires the selected Linux host")

func connectApplicationStream(context.Context, string, string) error {
	return errApplicationStreamHost
}

func publishApplicationStream(context.Context, string) error {
	return errApplicationStreamHost
}
