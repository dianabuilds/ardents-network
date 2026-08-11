package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/lab/directcontrol"
	"github.com/dianabuilds/ardents-network/internal/lab/preflight"
)

func directControl(arguments []string) int {
	return withRunLayout("direct-control", "owned Direct TLS session root", arguments, func(*flag.FlagSet) {}, func(layout preflight.RunLayout) int {
		binaryPath, err := os.Executable()
		if err != nil {
			return commandError("locate Carrier Lab binary", err)
		}
		ctx, stop := interruptContext()
		defer stop()
		evidence, err := directcontrol.RunControl(ctx, layout, binaryPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Direct TLS control: %v\nevidence: %s\n", err, evidence)
			return 2
		}
		fmt.Printf("Carrier Lab Direct TLS control: passed\nEvidence: %s\n", evidence)
		return 0
	})
}
func directRole(a []string) int   { return runDirectChild("direct-role", a, directcontrol.RunRole) }
func directTamper(a []string) int { return runDirectChild("direct-tamper", a, directcontrol.RunTamper) }
func runDirectChild(name string, arguments []string, execute func(context.Context, string, string) error) int {
	return runRoleCommand(name, arguments, func(ctx context.Context, config, evidence, _ string) error { return execute(ctx, config, evidence) })
}
