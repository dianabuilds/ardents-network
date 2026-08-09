package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/dianabuilds/ardents-network/internal/directcontrol"
	"github.com/dianabuilds/ardents-network/internal/harness"
	"github.com/dianabuilds/ardents-network/internal/preflight"
)

func directControl(arguments []string) int {
	flags := flag.NewFlagSet("direct-control", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", "", "repository root")
	sessionRoot := flags.String("session-root", "", "owned Direct TLS session root")
	tempRoot := flags.String("temp-root", "", "system temporary root")
	runID := flags.String("run-id", "", "run identifier")
	if err := flags.Parse(arguments); err != nil {
		return 64
	}
	layout, err := preflight.NewRunLayout(*sessionRoot, *repositoryRoot, *tempRoot, *runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Direct TLS layout: %v\n", err)
		return 2
	}
	binaryPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "locate Carrier Lab binary: %v\n", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	evidence, err := harness.RunDirectControl(ctx, layout, binaryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Direct TLS control: %v\nevidence: %s\n", err, evidence)
		return 2
	}
	fmt.Printf("Carrier Lab Direct TLS control: passed\nEvidence: %s\n", evidence)
	return 0
}

func directRole(arguments []string) int {
	return runDirectChild("direct-role", arguments, directcontrol.RunDirectRole)
}

func directTamper(arguments []string) int {
	return runDirectChild("direct-tamper", arguments, directcontrol.RunDirectTamper)
}

func runDirectChild(name string, arguments []string, execute func(context.Context, string, string) error) int {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	configPath := flags.String("config", "", "read-only role configuration")
	evidenceDir := flags.String("evidence-dir", "", "role-local evidence directory")
	if err := flags.Parse(arguments); err != nil {
		return 64
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := execute(ctx, *configPath, *evidenceDir); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		return 2
	}
	return 0
}
