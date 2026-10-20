//go:build !linux

package main

import "context"

func executeNetworkSigning(context.Context, []string) (*networkSigningResult, error) {
	return nil, errNetworkSigning
}
