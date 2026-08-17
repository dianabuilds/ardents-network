package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/lab/blockedentry"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(arguments []string) int {
	flags := flag.NewFlagSet("blocked-entry-lab", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	workspace := flags.String("workspace-root", "", "repository root excluded from evidence")
	evidence := flags.String("evidence-root", "", "new external evidence root")
	runID := flags.String("run-id", "", "immutable run identity")
	mode := flags.String("mode", "pass", "bounded development fixture mode")
	registry := flags.String("registry-root", "", "authoritative external consumed-run registry")
	runner := flags.String("runner", "", "separately built hostile-cell runner")
	verifier := flags.String("verifier", "", "pinned verifier executable")
	client := flags.String("client", "", "pinned WebTunnel client")
	server := flags.String("server", "", "pinned WebTunnel server")
	campaignSpec := flags.String("campaign-spec", "", "canonical frozen S5.5 campaign specification")
	prepareRoot := flags.String("prepare-final-root", "", "new external root for a frozen S5.5 specification")
	configurationRoot := flags.String("configuration-root", "", "private S5.5 configuration input root")
	linuxImage := flags.String("linux-image", "", "pinned Ubuntu LTS image identity")
	imageHash := flags.String("image-sha256", "", "pinned image SHA-256")
	kernel := flags.String("kernel", "", "pinned qualifying Linux kernel identity")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return 2
	}
	if *prepareRoot != "" {
		prepared, err := blockedentry.Run(blockedentry.Config{WorkspaceRoot: *workspace,
			PreparationRoot: *prepareRoot, ConfigurationRoot: *configurationRoot, LinuxImage: *linuxImage,
			ImageSHA256: *imageHash, Kernel: *kernel, ClientPath: *client, ServerPath: *server})
		if err != nil {
			fmt.Fprintln(os.Stderr, "blocked-entry-lab:", err)
			return 1
		}
		if err := json.NewEncoder(os.Stdout).Encode(prepared); err != nil {
			fmt.Fprintln(os.Stderr, "blocked-entry-lab:", err)
			return 1
		}
		return 0
	}
	result, err := blockedentry.Run(blockedentry.Config{WorkspaceRoot: *workspace, EvidenceRoot: *evidence,
		RunID: *runID, Mode: *mode, RegistryRoot: *registry, RunnerPath: *runner, VerifierPath: *verifier,
		ClientPath: *client, ServerPath: *server, CampaignSpecPath: *campaignSpec})
	if err != nil {
		fmt.Fprintln(os.Stderr, "blocked-entry-lab:", err)
		return 1
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "blocked-entry-lab:", err)
		return 1
	}
	return 0
}
