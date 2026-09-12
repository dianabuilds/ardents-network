//go:build !linux

package main

import (
	"context"
	"errors"
)

func runPublishedLink(context.Context, string) error {
	return errors.New("text Link presentation requires the selected Linux host")
}
