package main

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/admission/operator"
)

func runAdmission(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "holder", "receiver", "allocate", "issue-current":
			return runAdmissionLocal(ctx, args[0], args[1:], out, diagnostic)
		}
	}
	if len(args) > 0 && args[0] == "prepare-binding" {
		return operator.RunIssuerProfile(ctx, "admission.prepare-binding", args[1:], out, diagnostic)
	}
	return operator.RunAdmission(ctx, args, out, diagnostic)
}
