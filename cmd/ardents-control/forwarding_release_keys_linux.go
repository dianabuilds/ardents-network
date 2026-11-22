package main

import (
	"context"
	"io"
	"os/signal"
	"syscall"
	"time"
)

// A separate fixed initial Forwarding Release ceremony. No Endpoint receipt or
// fixture key is reused. This produces no signature or trust/independence claim.
func runForwardingReleaseKeys(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errSourceControlInput
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	public, err := prepareInfrastructureReleaseKeys(ctx, args[0], nil)
	if err != nil {
		return &forwardingControlFailure{err}
	}
	return writeReleaseKeyReceipt(ctx, out, "forwarding.prepare-release-keys", public)
}
