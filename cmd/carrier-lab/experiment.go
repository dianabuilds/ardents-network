package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/lab/preflight"
	"github.com/dianabuilds/ardents-network/internal/lab/routecomparison"
)

func routeExperiment(arguments []string) int {
	var applicationImage, toolImage, referenceDirectory string
	return withRunLayout("route-experiment", "owned R-013 experiment session root", arguments, func(flags *flag.FlagSet) {
		flags.StringVar(&applicationImage, "application-image", "", "immutable application image ID")
		flags.StringVar(&toolImage, "tool-image", "", "immutable tooling image ID")
		flags.StringVar(&referenceDirectory, "reference-directory", "", "pre-prepared pinned Tor/Chutney inputs")
	}, func(layout preflight.RunLayout) int {
		ctx, stop := interruptContext()
		defer stop()
		evidence, err := routecomparison.Run(ctx, layout, applicationImage, toolImage, referenceDirectory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "R-013 route experiment: %v\nevidence: %s\n", err, evidence)
			return 2
		}
		fmt.Printf("Carrier Lab R-013 comparative experiment: completed\nEvidence: %s\n", evidence)
		return 0
	})
}
