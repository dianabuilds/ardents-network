package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	var root string
	var format string

	flag.StringVar(&root, "root", ".", "repository root to scan")
	flag.StringVar(&format, "format", "text", "output format: text or json")
	flag.Parse()

	findings, err := scanRepository(root)
	if err != nil {
		fatal(err)
	}
	if len(findings) == 0 {
		return
	}

	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(findings); err != nil {
			fatal(err)
		}
	default:
		for _, finding := range findings {
			_, _ = fmt.Printf("%s imports %s outside network owner boundary\n", finding.File, finding.Import)
		}
	}

	os.Exit(1)
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
