package main

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/join"
)

// JOIN uses its separate declaration and original input lifetime. It never
// enters the legacy Node dispatcher or borrows Forwarding/Source permission.
func runJoinInstallation(parent context.Context, args []string, output io.Writer) error {
	if parent == nil || output == nil || len(args) != 2 || args[0] != "inspect-inputs" {
		return errors.New("usage: ardents-node join inspect-inputs REQUEST")
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	input, err := join.ReadInputs(ctx, args[1])
	if err == nil {
		err = join.InspectInitialInputs(input)
	}
	if err = errors.Join(err, input.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"join-inputs-preflight-completed\",\"installed\":false}\n")
	return errors.Join(err, ctx.Err())
}
