// Package main starts the Ardents operator command-line client.
// It does not own command behavior or remote node state.
package main

import (
	"context"
	"os"

	"ardents/internal/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
