package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"time"

	routesmoke "github.com/dianabuilds/ardents-network/internal/qualification/routesmoke"
	statequalification "github.com/dianabuilds/ardents-network/internal/qualification/state"
)

func evaluateRouteSmoke(arguments []string) (statequalification.Result, error) {
	flags := flag.NewFlagSet("route-smoke", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var fixture, evidence, compose, source string
	var duration time.Duration
	flags.StringVar(&fixture, "fixture", "", "new external Route fixture root")
	flags.StringVar(&evidence, "evidence", "", "new external Route evidence root")
	flags.StringVar(&compose, "compose", "", "Route smoke Compose file")
	flags.StringVar(&source, "source", "", "clean committed repository root")
	flags.DurationVar(&duration, "duration", 20*time.Minute, "10m..30m local development campaign")
	if err := flags.Parse(arguments); err != nil {
		return statequalification.Result{}, err
	}
	if flags.NArg() != 0 {
		return statequalification.Result{}, errors.New("route-smoke has unexpected positional arguments")
	}
	result := routesmoke.Run(context.Background(), routesmoke.Config{FixtureRoot: fixture, EvidenceRoot: evidence,
		ComposeFile: compose, SourceRoot: source, Duration: duration})
	return statequalification.Result{Verdict: result.Verdict, Reason: result.Reason, EvidenceRoot: result.EvidenceRoot,
		EvidenceDigest: result.EvidenceDigest}, nil
}
