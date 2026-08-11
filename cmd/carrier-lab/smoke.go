package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/lab/carrier"
	"github.com/dianabuilds/ardents-network/internal/lab/preflight"
	"github.com/dianabuilds/ardents-network/internal/lab/tooling"
)

func composeSmoke(arguments []string) int {
	return smokeControl(arguments, "compose-smoke", "Compose isolation", carrier.Run)
}
func toolingSmoke(arguments []string) int {
	return smokeControl(arguments, "tooling-smoke", "tooling", tooling.RunSmoke)
}

type smokeRunner func(context.Context, preflight.RunLayout, string, string) (string, error)

func smokeControl(arguments []string, name, label string, runSmoke smokeRunner) int {
	var image, fault string
	return withRunLayout(name, "owned smoke session root", arguments, func(flags *flag.FlagSet) {
		flags.StringVar(&image, "image", "", "immutable Carrier Lab image ID")
		flags.StringVar(&fault, "fault", "", "optional fixed fault injection")
	}, func(layout preflight.RunLayout) int {
		ctx, stop := interruptContext()
		defer stop()
		evidence, err := runSmoke(ctx, layout, image, fault)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s smoke: %v\nevidence: %s\n", label, err, evidence)
			return 2
		}
		fmt.Printf("Carrier Lab %s smoke: passed\nEvidence: %s\n", label, evidence)
		return 0
	})
}
func smokeRole(arguments []string) int {
	return runRoleCommand("smoke-role", arguments, func(_ context.Context, config, evidence, _ string) error { return carrier.RunRole(config, evidence) })
}
