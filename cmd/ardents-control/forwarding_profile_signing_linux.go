package main

import (
	"context"
	"io"
	"os"
)

// The Forwarding profile purpose selects only its own plan schema, decoder and
// State authority pin through the shared fixed ceremony.
func signForwardingProfile(ctx context.Context, args []string, observe func(string, *os.File)) (forwardingProfilePlan, []byte, error) {
	return signNodeProfile(ctx, args, observe, maximumForwardingProfilePlan, decodeForwardingProfilePlan)
}

func runForwardingProfileProvision(args []string, out io.Writer) error {
	return runNodeProfileProvision("forwarding", args, out, func(err error) error { return &forwardingControlFailure{err} }, prepareForwardingProfile, signForwardingProfile)
}
