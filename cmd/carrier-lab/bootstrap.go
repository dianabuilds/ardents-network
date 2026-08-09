package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

func bootstrap(arguments []string) int {
	flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", "", "repository root")
	goArchive := flags.String("go-archive", "", "absolute pinned Go archive path")
	seed := flags.String("seed", "", "optional deterministic seed")
	faultFinalizer := flags.Bool("fault-finalizer", false, "inject finalizer failure")
	if err := flags.Parse(arguments); err != nil {
		return 64
	}
	if flags.NArg() != 0 || *repositoryRoot == "" || *goArchive == "" {
		fmt.Fprintln(os.Stderr, "usage: carrier-lab bootstrap --repository-root ABSOLUTE_PATH --go-archive ABSOLUTE_PATH [--seed VALUE] [--fault-finalizer]")
		return 64
	}
	if err := preflight.Bootstrap(context.Background(), *repositoryRoot, *goArchive, *seed, *faultFinalizer, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap preflight: %v\n", err)
		return 2
	}
	return 0
}
