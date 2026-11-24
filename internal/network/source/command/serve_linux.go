//go:build linux

package command

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/network/source/serving"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func runSource(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) != 2 || (args[0] != "serve" && args[0] != "serve-retained") {
		return 2
	}
	switch serving.Run(ctx, args[1], args[0] == "serve-retained", Events(out, diagnostic)) {
	case serving.Canceled:
		return 130
	case serving.Refused:
		return 1
	default:
		return 2
	}
}

// Events projects bounded Source lifecycle events onto the CLI writers.
func Events(out, diagnostic io.Writer) func(context.Context, serving.Event) error {
	return func(ctx context.Context, event serving.Event) error {
		if event.Failure != nil {
			return serving.WriteFailureReport(diagnostic, event.Phase, event.Failure, event.ProcessClose)
		}
		if event.Phase == "listening" {
			return sourceReportContext(ctx, out, event.Phase, event.Outcome)
		}
		return sourceReport(diagnostic, event.Phase, event.Outcome)
	}
}

func sourceReport(out io.Writer, phase, outcome string) error {
	return sourceReportContext(context.Background(), out, phase, outcome)
}

func sourceReportContext(ctx context.Context, out io.Writer, phase, outcome string) error {
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{"source.serve", phase, outcome})
	if err != nil {
		return err
	}
	return artifacts.WriteReport(ctx, out, append(raw, '\n'))
}

func sourceStartupFailure(out io.Writer, owner string, cause, cleanup error) int {
	if serving.WriteFailureReport(out, "refused", &serving.Terminal{FirstOwner: owner, First: cause}, cleanup) != nil {
		return 2
	}
	return 1
}
