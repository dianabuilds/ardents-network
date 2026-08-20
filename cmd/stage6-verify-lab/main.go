package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/lab/stage6verify"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(arguments []string) int {
	flags := flag.NewFlagSet("stage6-verify-lab", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	manifest := flags.String("manifest-root", "", "read-only S6E1 manifest root")
	evidence := flags.String("evidence-root", "", "read-only S6E1 evidence root")
	private := flags.String("private-root", "", "read-only S6E1 private root")
	verdict := flags.String("verdict-root", "", "new verifier-owned verdict root")
	if flags.Parse(arguments) != nil || flags.NArg() != 0 || *manifest == "" || *evidence == "" || *private == "" || *verdict == "" {
		return 2
	}
	result := (stage6verify.Stage6Verifier{}).Verify(*manifest, *evidence, *private, *verdict)
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "stage6-verify-lab:", err)
		return 1
	}
	if result.Status != "pass" {
		return 1
	}
	return 0
}
