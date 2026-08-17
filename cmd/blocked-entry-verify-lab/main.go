package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/lab/blockedverify"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(arguments []string) int {
	flags := flag.NewFlagSet("blocked-entry-verify-lab", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	workspace := flags.String("workspace-root", "", "repository root for final supply verification")
	manifest := flags.String("manifest", "", "immutable manifest")
	evidence := flags.String("evidence", "", "canonical publishable evidence")
	closure := flags.String("closure", "", "canonical evidence closure")
	secretRoot := flags.String("secret-root", "", "read-only secret artifacts")
	registry := flags.String("registry-root", "", "external consumed-run registry")
	canaries := flags.String("canaries", "", "private four-canary corpus")
	publishable := flags.String("publishable-root", "", "complete publishable tree")
	output := flags.String("output", "", "canonical verifier result")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return 2
	}
	result, err := blockedverify.Verify(blockedverify.Config{WorkspaceRoot: *workspace,
		ManifestPath: *manifest, EvidencePath: *evidence,
		ClosurePath: *closure, SecretRoot: *secretRoot, RegistryRoot: *registry,
		CanaryPath: *canaries, PublishableRoot: *publishable, OutputPath: *output})
	if err != nil {
		fmt.Fprintln(os.Stderr, "blocked-entry-verify-lab:", err)
		return 1
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "blocked-entry-verify-lab:", err)
		return 1
	}
	return 0
}
