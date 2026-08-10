package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/harness/tooling"
	"github.com/dianabuilds/ardents-network/internal/nativecircuit"
	"github.com/dianabuilds/ardents-network/internal/preflight"
)

func nativeRun(arguments []string) int {
	var applicationImage, toolImage, fault string
	return withRunLayout("native-run", "owned native-run session root", arguments, func(flags *flag.FlagSet) {
		flags.StringVar(&applicationImage, "application-image", "", "immutable application image ID")
		flags.StringVar(&toolImage, "tool-image", "", "immutable tooling image ID")
		flags.StringVar(&fault, "fault", "", "optional fixed fault: rendezvous-process")
	}, func(layout preflight.RunLayout) int {
		ctx, stop := interruptContext()
		defer stop()
		evidence, err := nativecircuit.Run(ctx, layout, applicationImage, toolImage, fault)
		if err != nil {
			fmt.Fprintf(os.Stderr, "native C-5/C2 run: %v\nevidence: %s\n", err, evidence)
			return 2
		}
		fmt.Printf("Carrier Lab native C-5/C2 development smoke: passed\nEvidence: %s\n", evidence)
		return 0
	})
}
func nativeRole(arguments []string) int {
	return runRoleCommand("native-role", arguments, func(ctx context.Context, config, evidence, _ string) error {
		return nativecircuit.RunRole(ctx, config, evidence)
	})
}
func nativeToolRole(arguments []string) int {
	return runRoleCommand("native-tool-role", arguments, func(_ context.Context, config, evidence, capture string) error {
		return tooling.RunNativeRole(config, evidence, capture)
	})
}
