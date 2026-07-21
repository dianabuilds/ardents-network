package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	var tags string
	var layer string
	var domain string
	var scenario string
	var tag string
	var suite string

	flag.StringVar(&tags, "tags", "", "space-separated go build tags")
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
	runCatalog(encoder, tags, patterns, layer, domain, scenario, tag, suite)
}

func runCatalog(encoder *json.Encoder, tags string, patterns []string, layer string, domain string, scenario string, tag string, suite string) {
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

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
