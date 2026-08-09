package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/dianabuilds/ardents-network/internal/directcontrol"
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
		return commandError("Direct TLS layout", err)
	}
	binaryPath, err := os.Executable()
	if err != nil {
		return commandError("locate Carrier Lab binary", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	evidence, err := directcontrol.RunControl(ctx, layout, binaryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Direct TLS control: %v\nevidence: %s\n", err, evidence)
		return 2
	}
	fmt.Printf("Carrier Lab Direct TLS control: passed\nEvidence: %s\n", evidence)
	return 0
}

func directRole(arguments []string) int {
	return runDirectChild("direct-role", arguments, directcontrol.RunRole)
}

func directTamper(arguments []string) int {
	return runDirectChild("direct-tamper", arguments, directcontrol.RunTamper)
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
		return commandError(name, err)
	}
	return 0
}
