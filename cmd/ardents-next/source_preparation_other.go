//go:build !linux

package main

import (
	"context"
	"io"
)

func runSourcePreparation(context.Context, []string, io.Writer) int { return 2 }
