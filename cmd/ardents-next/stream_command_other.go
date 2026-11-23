//go:build !linux

package main

import (
	"context"
	"errors"
)

func connectApplicationStream(context.Context, string, string) error {
	return errors.New("Application stream requires the selected Linux host")
}
