package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"os/signal"

	nodequalification "github.com/dianabuilds/ardents-network/internal/qualification/node"
	statequalification "github.com/dianabuilds/ardents-network/internal/qualification/state"
)

func evaluateNodeCampaign(arguments []string) (statequalification.Result, error) {
	flags := flag.NewFlagSet("run-node", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var fixture, evidence, compose, mode string
	flags.StringVar(&fixture, "fixture", "", "prepared node fixture root")
	flags.StringVar(&evidence, "evidence", "", "new external evidence root")
	flags.StringVar(&compose, "compose", "", "node Compose file")
	flags.StringVar(&mode, "mode", "short", "short hostile-resource matrix")
	if err := flags.Parse(arguments); err != nil {
		return statequalification.Result{}, err
	}
	if flags.NArg() != 0 {
		return statequalification.Result{}, errors.New("run-node has unexpected positional arguments")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return renderNode(nodequalification.Run(ctx, nodequalification.Campaign{
		FixtureRoot: fixture, EvidenceRoot: evidence, ComposeFile: compose, Mode: mode,
	})), nil
}
