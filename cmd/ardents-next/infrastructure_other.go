//go:build !linux

package main

import (
	"context"
	"io"
)

func runInfrastructure(context.Context, []string, io.Writer) int { return 2 }
