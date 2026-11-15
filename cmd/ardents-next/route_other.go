//go:build !linux

package main

import (
	"context"
	"io"
)

func runRoute(context.Context, []string, io.Writer, io.Writer) int { return 1 }
