package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/dianabuilds/ardents-network/internal/harness"
	"github.com/dianabuilds/ardents-network/internal/harness/tooling"
	"github.com/dianabuilds/ardents-network/internal/preflight"
)

func composeSmoke(arguments []string) int {
	return smokeControl(arguments, "compose-smoke", "Compose isolation", harness.Run)
}
func toolingSmoke(arguments []string) int {
	return smokeControl(arguments, "tooling-smoke", "tooling", tooling.RunSmoke)
}

type smokeRunner func(context.Context, preflight.RunLayout, string, string) (string, error)

func smokeControl(arguments []string, name, label string, runSmoke smokeRunner) int {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
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
		fmt.Fprintf(os.Stderr, "%s layout: %v\n", label, err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	evidence, err := runSmoke(ctx, layout, *image, *fault)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s smoke: %v\nevidence: %s\n", label, err, evidence)
		return 2
	}
	fmt.Printf("Carrier Lab %s smoke: passed\nEvidence: %s\n", label, evidence)
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
