package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	var root string
	var format string
	var failOnUpdate bool

	flag.StringVar(&root, "root", ".", "repository root")
	flag.StringVar(&format, "format", "text", "output format: text or json")
	flag.BoolVar(&failOnUpdate, "fail-on-update", false, "exit with code 1 when a newer upstream stable tag exists")
	flag.Parse()

	manifests, err := loadForkManifests(filepath.Join(root, "third_party", "forks"))
	if err != nil {
		fatal(err)
	}

	results := make([]Result, 0, len(manifests))
	hasUpdate := false
	for _, manifest := range manifests {
		result, err := checkManifest(manifest)
		if err != nil {
			fatal(err)
		}
		if result.Status == StatusUpdateAvailable {
			hasUpdate = true
		}
		results = append(results, result)
	}

	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			fatal(err)
		}
	default:
		for _, result := range results {
			fmt.Printf("%s: pinned=%s latest=%s status=%s\n", result.Name, result.PinnedBaseline, result.LatestStableTag, result.Status)
			if len(result.NewerStableTags) > 0 {
				fmt.Printf("  newer stable tags: %v\n", result.NewerStableTags)
			}
		}
	}

	if failOnUpdate && hasUpdate {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
