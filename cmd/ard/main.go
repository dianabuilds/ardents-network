package main

import (
	"context"
	"os"

	"ardents/boundary/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
