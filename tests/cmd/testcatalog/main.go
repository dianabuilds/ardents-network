package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	var tags string
	var mode string
	var layer string
	var domain string
	var scenario string
	var tag string
	var suite string

	flag.StringVar(&tags, "tags", "", "space-separated go build tags")
	flag.StringVar(&mode, "mode", "catalog", "output mode: catalog, inventory, validate")
	flag.StringVar(&layer, "layer", "", "filter by layer")
	flag.StringVar(&domain, "domain", "", "filter by domain")
	flag.StringVar(&scenario, "scenario", "", "filter by scenario id")
	flag.StringVar(&tag, "tag", "", "filter by tag")
	flag.StringVar(&suite, "suite", "", "filter by suite/profile")
	flag.Parse()

	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./tests/..."}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	switch mode {
	case "catalog":
		runCatalogMode(encoder, tags, patterns, layer, domain, scenario, tag, suite)
	case "inventory", "validate":
		runInventoryMode(encoder, mode, patterns, layer, domain, scenario)
	default:
		fatal(fmt.Errorf("unsupported mode %q", mode))
	}
}

func runCatalogMode(encoder *json.Encoder, tags string, patterns []string, layer string, domain string, scenario string, tag string, suite string) {
	packages, err := listPackages(tags, patterns)
	if err != nil {
		fatal(err)
	}

	entries, err := buildCatalog(packages)
	if err != nil {
		fatal(err)
	}

	entries = filterCatalog(entries, layer, domain, scenario, tag, suite)
	if err := encoder.Encode(entries); err != nil {
		fatal(err)
	}
}

func runInventoryMode(encoder *json.Encoder, mode string, patterns []string, layer string, domain string, scenario string) {
	report, err := buildInventory(patterns)
	if err != nil {
		fatal(err)
	}

	report = filterInventory(report, layer, domain, scenario)
	if err := encoder.Encode(report); err != nil {
		fatal(err)
	}
	if mode == "validate" && report.Summary.IssueCount > 0 {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
