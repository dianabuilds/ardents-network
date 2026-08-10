// Command reference-site-lab is the thin executable adapter for the maintained
// Gate C experiment and its closed container roles.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/experimentrun"
	"github.com/dianabuilds/ardents-network/internal/siteexperiment"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(arguments []string) int {
	if len(arguments) == 0 {
		fmt.Fprintln(os.Stderr, "usage: reference-site-lab run|role [flags]")
		return 2
	}
	if arguments[0] == "run" {
		return runGateC(arguments[1:])
	}
	if arguments[0] != "role" {
		return 2
	}
	flags := flag.NewFlagSet("role", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	role := flags.String("role", "", "closed Gate C role")
	socket := flags.String("socket", "", "owned Unix socket path")
	nonce := flags.String("nonce", "", "32-byte canonical hex nonce")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 {
		return 2
	}
	if err := siteexperiment.RunRole(context.Background(), *role, siteexperiment.RoleConfig{SocketPath: *socket, NonceHex: *nonce}); err != nil {
		fmt.Fprintf(os.Stderr, "reference-site-lab: %v\n", err)
		return 1
	}
	return 0
}

func runGateC(arguments []string) int {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	sessionRoot := flags.String("session-root", "", "owned experiment session")
	repositoryRoot := flags.String("repository-root", "", "repository root")
	temporaryRoot := flags.String("temporary-root", "", "system temporary root")
	runID := flags.String("run-id", "", "fixed run identity")
	applicationImage := flags.String("application-image", "", "immutable Carrier Application image ID")
	toolImage := flags.String("tool-image", "", "immutable Carrier tooling image ID")
	referenceImage := flags.String("reference-image", "", "immutable Reference Site image ID")
	r013Receipt := flags.String("r013-receipt", "", "bounded advancing R-013 regression receipt")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return 2
	}
	identity, err := experimentrun.New(*sessionRoot, *repositoryRoot, *temporaryRoot, *runID)
	if err == nil {
		_, err = siteexperiment.Run(context.Background(), identity, *applicationImage, *toolImage, *referenceImage, *r013Receipt)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "reference-site-lab: %v\n", err)
		return 1
	}
	return 0
}
