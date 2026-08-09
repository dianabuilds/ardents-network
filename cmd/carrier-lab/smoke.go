package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/dianabuilds/ardents-network/internal/harness"
	"github.com/dianabuilds/ardents-network/internal/preflight"
)

func composeSmoke(arguments []string) int {
	flags := flag.NewFlagSet("compose-smoke", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", "", "repository root")
	sessionRoot := flags.String("session-root", "", "owned smoke session root")
	tempRoot := flags.String("temp-root", "", "system temporary root")
	runID := flags.String("run-id", "", "run identifier")
	image := flags.String("image", "", "immutable Carrier Lab image ID")
	fault := flags.String("fault", "", "optional fixed fault injection")
	if err := flags.Parse(arguments); err != nil {
		return 64
	}
	layout, err := preflight.NewRunLayout(*sessionRoot, *repositoryRoot, *tempRoot, *runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compose smoke layout: %v\n", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	evidence, err := harness.Run(ctx, layout, *image, *fault)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compose smoke: %v\nevidence: %s\n", err, evidence)
		return 2
	}
	fmt.Printf("Carrier Lab Compose smoke: passed\nEvidence: %s\n", evidence)
	return 0
}

func smokeRole(arguments []string) int {
	flags := flag.NewFlagSet("smoke-role", flag.ContinueOnError)
	configPath := flags.String("config", "", "read-only role configuration")
	evidenceDir := flags.String("evidence-dir", "", "role-local evidence directory")
	if err := flags.Parse(arguments); err != nil {
		return 64
	}
	if err := harness.RunRole(*configPath, *evidenceDir); err != nil {
		fmt.Fprintf(os.Stderr, "smoke role: %v\n", err)
		return 2
	}
	return 0
}
