// Command named-site-lab is the thin executable adapter for the maintained
// Gate C experiment and its closed container roles.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/lab/namedsite"
	"github.com/dianabuilds/ardents-network/internal/lab/runlayout"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(arguments []string) int {
	if len(arguments) == 0 {
		fmt.Fprintln(os.Stderr, "usage: named-site-lab run|role [flags]")
		return 2
	}
	if arguments[0] == "run" {
		return runGateC(arguments[1:])
	}
	if arguments[0] == "probe" {
		flags := flag.NewFlagSet("probe", flag.ContinueOnError)
		kind := flags.String("kind", "", "closed isolation probe")
		observerName := flags.String("observer-name", "", "controlled forbidden-boundary observer name")
		observerAddress := flags.String("observer-address", "", "controlled forbidden-boundary observer address")
		if flags.Parse(arguments[1:]) != nil || flags.NArg() != 0 {
			return 2
		}
		if err := namedsite.RunRole(context.Background(), "isolation-probe", namedsite.RoleConfig{ProbeKind: *kind, ObserverName: *observerName, ObserverAddress: *observerAddress}); err != nil {
			fmt.Fprintf(os.Stderr, "named-site-lab: %v\n", err)
			return 1
		}
		return 0
	}
	if arguments[0] != "role" {
		return 2
	}
	flags := flag.NewFlagSet("role", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	role := flags.String("role", "", "closed Gate C role")
	socket := flags.String("socket", "", "owned Unix socket path")
	gatewaySocket := flags.String("gateway-socket", "", "role-local Gateway Unix socket path")
	nonce := flags.String("nonce", "", "32-byte canonical hex nonce")
	configPath := flags.String("config", "", "role-local configuration")
	evidenceDir := flags.String("evidence-dir", "", "role-local bounded evidence")
	observerName := flags.String("observer-name", "", "controlled forbidden-boundary observer name")
	observerAddress := flags.String("observer-address", "", "controlled forbidden-boundary observer address")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 {
		return 2
	}
	if err := namedsite.RunRole(context.Background(), *role, namedsite.RoleConfig{SocketPath: *socket, GatewaySocketPath: *gatewaySocket, NonceHex: *nonce, ConfigPath: *configPath, EvidenceDir: *evidenceDir, ObserverName: *observerName, ObserverAddress: *observerAddress}); err != nil {
		fmt.Fprintf(os.Stderr, "named-site-lab: %v\n", err)
		return 1
	}
	return 0
}

func runGateC(arguments []string) int {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	sessionRoot := flags.String("session-root", "", "owned experiment session")
	repositoryRoot := flags.String("repository-root", "", "repository root")
	temporaryRoot := flags.String("temporary-root", "", "system temporary root")
	runID := flags.String("run-id", "", "fixed run identity")
	applicationImage := flags.String("application-image", "", "immutable Carrier Application image ID")
	toolImage := flags.String("tool-image", "", "immutable Carrier tooling image ID")
	referenceImage := flags.String("reference-image", "", "immutable Reference Site image ID")
	r013Receipt := flags.String("r013-receipt", "", "bounded advancing R-013 regression receipt")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return 2
	}
	identity, err := runlayout.New(*sessionRoot, *repositoryRoot, *temporaryRoot, *runID)
	if err == nil {
		_, err = namedsite.Run(context.Background(), identity, *applicationImage, *toolImage, *referenceImage, *r013Receipt)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "named-site-lab: %v\n", err)
		return 1
	}
	return 0
}
