package main

import (
	"context"
	"io"
	"os"
)

// The Descriptor profile purpose selects only its own plan schema, decoder and
// State authority pin through the shared fixed ceremony.
func signDescriptorProfile(ctx context.Context, args []string, observe func(string, *os.File)) (descriptorProfilePlan, []byte, error) {
	return signNodeProfile(ctx, args, observe, maximumDescriptorProfilePlan, decodeDescriptorProfilePlan)
}

func runDescriptorProfileProvision(args []string, out io.Writer) error {
	return runNodeProfileProvision("descriptor", args, out, func(err error) error { return &descriptorControlFailure{err} }, prepareDescriptorProfile, signDescriptorProfile)
}
