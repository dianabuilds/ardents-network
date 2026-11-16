//go:build !linux && !darwin

package main

import (
	"context"
	"io"
)

func writeNetworkPreparationReceipt(context.Context, io.Writer, []byte) error {
	return errNetworkPreparation
}
