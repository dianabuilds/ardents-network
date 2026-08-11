package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/lab/nativecircuit"
	"github.com/dianabuilds/ardents-network/internal/lab/preflight"
	"github.com/dianabuilds/ardents-network/internal/lab/tooling"
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

func nativeNegative(arguments []string) int {
	flags := flag.NewFlagSet("native-negative", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var name string
	flags.StringVar(&name, "case", "", "fixed R-013 negative case")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return 64
	}
	ctx, stop := interruptContext()
	defer stop()
	if err := nativecircuit.RunNegative(ctx, name); err != nil {
		return commandError("native negative "+name, err)
	}
	fmt.Printf("Carrier Lab negative %s: failed closed\n", name)
	return 0
}
