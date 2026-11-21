//go:build linux

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) > 0 {
		switch arguments[0] {
		case "verify-pair":
			return verifyPair(arguments[1:], output)
		case "verify-network-manifest":
			return verifyNetworkManifest(arguments[1:], output)
		case "verify-net14v":
			return verifyNET14V(arguments[1:], output)
		case "verify-failed-net14v":
			return verifyFailedNET14V(arguments[1:], output)
		case "verify-run":
			return verifyCompletedRun(arguments[1:], output)
		}
	}
	return runQualification(ctx, arguments, output)
}
