//go:build !linux

package main

import (
	"context"
	"io"
)

func readNetworkPreparation(context.Context, string) ([]byte, error) {
	return nil, errNetworkPreparation
}
func writeNetworkPreparation(context.Context, string, string, []byte) error {
	return errNetworkPreparation
}
func writeNetworkPreparationReceipt(context.Context, io.Writer, []byte) error {
	return errNetworkPreparation
}
