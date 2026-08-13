package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	qualification "github.com/dianabuilds/ardents-network/internal/qualification/route"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(arguments []string, output, diagnostics io.Writer) int {
	result := qualification.Result{Verdict: "invalid", Reason: "usage: ardents-route-qualify <manifest.json>"}
	if len(arguments) == 1 {
		input, err := readCase(arguments[0])
		if err != nil {
			result.Reason = err.Error()
		} else {
			result = qualification.Evaluate(input)
		}
	}
	if err := json.NewEncoder(output).Encode(result); err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	switch result.Verdict {
	case "pass":
		return 0
	case "fail":
		return 1
	default:
		return 2
	}
}
