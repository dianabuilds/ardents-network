package main

import (
	"context"
	"flag"
	"os"
	"os/signal"

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

func interruptContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

func withRunLayout(name, session string, arguments []string, configure func(*flag.FlagSet), execute func(preflight.RunLayout) int) int {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", "", "repository root")
	sessionRoot := flags.String("session-root", "", session)
	tempRoot := flags.String("temp-root", "", "system temporary root")
	runID := flags.String("run-id", "", "run identifier")
	configure(flags)
	if err := flags.Parse(arguments); err != nil {
		return 64
	}
	layout, err := preflight.NewRunLayout(*sessionRoot, *repositoryRoot, *tempRoot, *runID)
	if err != nil {
		return commandError(name+" layout", err)
	}
	return execute(layout)
}
func runRoleCommand(name string, arguments []string, execute func(context.Context, string, string, string) error) int {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	configPath := flags.String("config", "", "read-only role configuration")
	evidenceDir := flags.String("evidence-dir", "", "role-local evidence directory")
	captureDir := flags.String("capture-dir", "/tmp", "owned capture directory")
	if err := flags.Parse(arguments); err != nil {
		return 64
	}
	ctx, stop := interruptContext()
	defer stop()
	if err := execute(ctx, *configPath, *evidenceDir, *captureDir); err != nil {
		return commandError(name, err)
	}
	return 0
}
