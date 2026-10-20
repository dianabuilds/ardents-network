//go:build !linux

package main

import (
	"context"
	"io"
)

func runSource(context.Context, []string, io.Writer, io.Writer) int { return 2 }
