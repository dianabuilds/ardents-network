//go:build linux

package main

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/network/source/serving"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
)

type sourceTerminal = serving.Terminal

func runSource(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) != 2 || (args[0] != "serve" && args[0] != "serve-retained") {
		return 2
	}
	switch serving.Run(ctx, args[1], args[0] == "serve-retained", sourceEvents(out, diagnostic)) {
	case serving.Canceled:
		return 130
	case serving.Refused:
		return 1
	default:
		return 2
	}
}

func sourceEvents(out, diagnostic io.Writer) func(context.Context, serving.Event) error {
	return func(ctx context.Context, event serving.Event) error {
		if event.Failure != nil {
			return sourceFailureReport(diagnostic, event.Phase, event.Failure, event.ProcessClose)
		}
		if event.Phase == "listening" {
			return sourceReportContext(ctx, out, event.Phase, event.Outcome)
		}
		return sourceReport(diagnostic, event.Phase, event.Outcome)
	}
}

func validSourceServePlan(plan servingdeclaration.ServingDeclaration) bool {
	return serving.ValidDeclaration(plan)
}
