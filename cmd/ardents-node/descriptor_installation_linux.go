package main

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor"
)

// Inspection retains its own original delivery and read-only manager. Neither
// its result nor prepared public bytes admit installation or runtime effects.
func runDescriptorInstallation(parent context.Context, args []string, output io.Writer) error {
	if parent == nil || output == nil || len(args) != 2 || args[0] != "inspect-inputs" {
		return errors.New("usage: ardents-node descriptor inspect-inputs REQUEST")
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	input, err := descriptor.ReadInputs(ctx, args[1])
	if err == nil {
		err = descriptor.InspectInitialInputs(input)
	}
	if err = errors.Join(err, input.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"descriptor-inputs-preflight-completed\",\"installed\":false}\n")
	return errors.Join(err, ctx.Err())
}
