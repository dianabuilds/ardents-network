//go:build !linux

package main

import (
	"context"
	"io"
)

func runPlatform(context.Context, []string, io.Writer) int { return 2 }
