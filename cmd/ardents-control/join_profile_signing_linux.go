package main

import (
	"context"
	"io"
	"os"
)

// The JOIN profile purpose selects only its own plan schema, decoder and State
// authority pin through the shared fixed ceremony.
func signJoinProfile(ctx context.Context, args []string, observe func(string, *os.File)) (joinProfilePlan, []byte, error) {
	return signNodeProfile(ctx, args, observe, maximumJoinProfilePlan, decodeJoinProfilePlan)
}

func runJoinProfileProvision(args []string, out io.Writer) error {
	return runNodeProfileProvision("join", args, out, func(err error) error { return &joinControlFailure{err} }, prepareJoinProfile, signJoinProfile)
}
