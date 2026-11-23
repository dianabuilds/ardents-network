//go:build !linux

package main

import (
	"context"
	"errors"
	"io"
)

func writeNetworkPreparationReceipt(context.Context, io.Writer, []byte) error {
	return errors.New("network public preparation unavailable")
}
