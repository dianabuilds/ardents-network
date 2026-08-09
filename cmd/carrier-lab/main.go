// Command carrier-lab drives fixed lab commands and records bounded evidence and cleanup verdicts.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

func main() {
	os.Exit(run(os.Args[1:]))
}
func commandError(label string, err error) int {
	fmt.Fprintf(os.Stderr, "%s: %v\n", label, err)
	return 2
}
func run(arguments []string) int {
	if len(arguments) == 0 {
		fmt.Fprintln(os.Stderr, "usage: carrier-lab <bootstrap|evaluate|finalize-cleanup|compose-smoke|smoke-role|tooling-verify|tooling-smoke|tooling-role|direct-control|direct-role|direct-tamper> [options]")
		return 64
	}
	switch arguments[0] {
	case "bootstrap":
		return bootstrap(arguments[1:])
	case "evaluate":
		return evaluate(arguments[1:])
	case "finalize-cleanup":
		return finalizeCleanup(arguments[1:])
	case "compose-smoke":
		return composeSmoke(arguments[1:])
	case "smoke-role":
		return smokeRole(arguments[1:])
	case "tooling-verify":
		return toolingVerify(arguments[1:])
	case "tooling-smoke":
		return toolingSmoke(arguments[1:])
	case "tooling-role":
		return toolingRole(arguments[1:])
	case "direct-control":
		return directControl(arguments[1:])
	case "direct-role":
		return directRole(arguments[1:])
	case "direct-tamper":
		return directTamper(arguments[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", arguments[0])
		return 64
	}
}

func evaluate(arguments []string) int {
	flags := flag.NewFlagSet("evaluate", flag.ContinueOnError)
	inputPath := flags.String("input", "", "validated orchestrator input")
	repositoryRoot := flags.String("repository-root", "", "read-only repository root")
	sessionRoot := flags.String("session-root", "", "owned preflight session root")
	tempRoot := flags.String("temp-root", "", "system temporary root")
	runID := flags.String("run-id", "", "run identifier")
	if err := flags.Parse(arguments); err != nil {
		return 64
	}
	layout, err := preflight.NewRunLayout(*sessionRoot, *repositoryRoot, *tempRoot, *runID)
	if err != nil {
		return commandError("evaluate preflight layout", err)
	}
	result, err := preflight.Evaluate(*inputPath, layout)
	if err != nil {
		return commandError("evaluate preflight", err)
	}
	if !result.ChecksPassed {
		return 2
	}
	return 0
}

func finalizeCleanup(arguments []string) int {
	flags := flag.NewFlagSet("finalize-cleanup", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", "", "repository root")
	sessionRoot := flags.String("session-root", "", "owned preflight session root")
	tempRoot := flags.String("temp-root", "", "system temporary root")
	runID := flags.String("run-id", "", "run identifier")
	containersAbsent := flags.Bool("owned-containers-absent", false, "whether owned containers are absent")
	networksAbsent := flags.Bool("owned-networks-absent", false, "whether owned networks are absent")
	volumesAbsent := flags.Bool("owned-volumes-absent", false, "whether owned volumes are absent")
	if err := flags.Parse(arguments); err != nil {
		return 64
	}
	layout, err := preflight.NewRunLayout(*sessionRoot, *repositoryRoot, *tempRoot, *runID)
	if err != nil {
		return commandError("finalize cleanup layout", err)
	}
	result, err := preflight.FinalizeCleanup(layout, preflight.OwnedResources{
		ContainersAbsent: *containersAbsent,
		NetworksAbsent:   *networksAbsent,
		VolumesAbsent:    *volumesAbsent,
	})
	if err != nil {
		return commandError("finalize cleanup", err)
	}
	if !result.Passed {
		return 2
	}
	return 0
}
