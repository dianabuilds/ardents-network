//go:build !linux

package main

import "context"

func executeInitialKeys(context.Context, string, string) (*initialKeyReceipt, error) {
	return nil, errInitialKeys
}
