// Command carrier-lab drives fixed lab commands and records bounded evidence and cleanup verdicts.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

func main() { os.Exit(run(os.Args[1:])) }
func commandError(label string, err error) int {
	fmt.Fprintf(os.Stderr, "%s: %v\n", label, err)
	return 2
}
func run(arguments []string) int {
	if len(arguments) == 0 {
		fmt.Fprintln(os.Stderr, "usage: carrier-lab <bootstrap|evaluate|finalize-cleanup|compose-smoke|smoke-role|tooling-verify|tooling-smoke|tooling-role|direct-control|direct-role|direct-tamper|native-run|native-role|native-tool-role> [options]")
		return 64
	}
	commands := map[string]func([]string) int{
		"bootstrap": bootstrap, "evaluate": evaluate, "finalize-cleanup": finalizeCleanup,
		"compose-smoke": composeSmoke, "smoke-role": smokeRole,
		"tooling-verify": toolingVerify, "tooling-smoke": toolingSmoke, "tooling-role": toolingRole,
		"direct-control": directControl, "direct-role": directRole, "direct-tamper": directTamper,
		"native-run": nativeRun, "native-role": nativeRole, "native-tool-role": nativeToolRole,
	}
	if command := commands[arguments[0]]; command != nil {
		return command(arguments[1:])
	}
	fmt.Fprintf(os.Stderr, "unknown command %q\n", arguments[0])
	return 64
}
func evaluate(arguments []string) int {
	var inputPath string
	return withRunLayout("evaluate", "owned preflight session root", arguments,
		func(flags *flag.FlagSet) { flags.StringVar(&inputPath, "input", "", "validated orchestrator input") },
		func(layout preflight.RunLayout) int {
			result, err := preflight.Evaluate(inputPath, layout)
			if err != nil {
				return commandError("evaluate preflight", err)
			}
			if !result.ChecksPassed {
				return 2
			}
			return 0
		})
}
func finalizeCleanup(arguments []string) int {
	var resources preflight.OwnedResources
	return withRunLayout("finalize-cleanup", "owned preflight session root", arguments,
		func(flags *flag.FlagSet) {
			flags.BoolVar(&resources.ContainersAbsent, "owned-containers-absent", false, "owned containers are absent")
			flags.BoolVar(&resources.NetworksAbsent, "owned-networks-absent", false, "owned networks are absent")
			flags.BoolVar(&resources.VolumesAbsent, "owned-volumes-absent", false, "owned volumes are absent")
		}, func(layout preflight.RunLayout) int {
			result, err := preflight.FinalizeCleanup(layout, resources)
			if err != nil {
				return commandError("finalize cleanup", err)
			}
			if !result.Passed {
				return 2
			}
			return 0
		})
}
