package main

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/admission/operator"
)

func runIssuance(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 && args[0] == "inspect-public-profile" {
		return runIssuerPublicInspection(ctx, args[1:], out)
	}
	return operator.RunIssuance(ctx, args, out, diagnostic)
}
