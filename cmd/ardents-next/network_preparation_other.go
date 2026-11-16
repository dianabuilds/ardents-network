//go:build !linux

package main

import (
	"context"
)

func readNetworkPreparation(context.Context, string) ([]byte, error) {
	return nil, errNetworkPreparation
}
func writeNetworkPreparation(context.Context, string, string, []byte) error {
	return errNetworkPreparation
}
