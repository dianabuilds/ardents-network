package main

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/console"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
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
		local, err := console.Open(ctx, os.Stdin, out, diagnostic)
		if err != nil {
			return 1
		}
		defer local.Close()
		if operation == "holder" {
			return runAdmissionHolder(local.Context, args, local.Input, local.Output, local.Diagnostic)
		}
		if operation == "execution-holder" {
			return runExecutionPreparation(local.Context, args, local.Input, local.Output, local.Diagnostic)
		}
		if operation == "execution-holder-live" {
			return runExecutionRouteHolder(local.Context, args, local.Input, local.Output, local.Diagnostic)
		}
		return runAdmissionReceiver(local.Context, args, local.Input, local.Output, local.Diagnostic)
	}
	switch operation {
	case "allocate":
		return runAdmissionAllocation(ctx, args, out)
	case "issue-current":
		return runAdmissionCurrentIssuer(ctx, args, out)
	}
	return 2
}
