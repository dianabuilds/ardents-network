package main

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
	"os"
)

func decodeAdmissionObject(raw []byte, value any) error { return artifacts.DecodeObject(raw, value) }
func admissionConfig(args []string, value any) error {
	return admissionConfigBounded(args, value, 64<<10)
}
func admissionConfigBounded(args []string, value any, maximum int64) error {
	if len(args) != 2 || args[0] != "--config" {
		return errors.New("invalid arguments")
	}
	raw, err := readBounded(args[1], maximum)
	if err != nil {
		return err
	}
	return decodeAdmissionObject(raw, value)
}
func runAdmissionLocal(ctx context.Context, operation string, args []string, out, diagnostic io.Writer) int {
	if ctx == nil {
		return 2
	}
	if operation == "holder" || operation == "receiver" || operation == "execution-holder" || operation == "execution-holder-live" {
		console, err := admission.OpenLocalConsole(ctx, os.Stdin, out, diagnostic)
		if err != nil {
			return 1
		}
		defer console.Close()
		if operation == "holder" {
			return runAdmissionHolder(console.Context, args, console.Input, console.Output, console.Diagnostic)
		}
		if operation == "execution-holder" {
			return runExecutionPreparation(console.Context, args, console.Input, console.Output, console.Diagnostic)
		}
		if operation == "execution-holder-live" {
			return runExecutionRouteHolder(console.Context, args, console.Input, console.Output, console.Diagnostic)
		}
		return runAdmissionReceiver(console.Context, args, console.Input, console.Output, console.Diagnostic)
	}
	switch operation {
	case "allocate":
		return runAdmissionAllocation(ctx, args, out)
	case "issue-current":
		return runAdmissionCurrentIssuer(ctx, args, out)
	}
	return 2
}
