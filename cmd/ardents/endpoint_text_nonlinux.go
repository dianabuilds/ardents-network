//go:build !linux

package main

import (
	"context"
	"errors"
	"io"
)

func runTextHeadlessRuntime(context.Context, decodedHeadlessRuntimePlan, io.Writer) error {
	return errors.New("text runtime plan requires the qualified Linux installation")
}
