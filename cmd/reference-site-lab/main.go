// Command reference-site-lab is the thin executable adapter for the maintained
// Gate C experiment and its closed container roles.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/siteexperiment"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(arguments []string) int {
	if len(arguments) == 0 || arguments[0] != "role" {
		fmt.Fprintln(os.Stderr, "usage: reference-site-lab role --role NAME --socket PATH --nonce HEX")
		return 2
	}
	flags := flag.NewFlagSet("role", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	role := flags.String("role", "", "closed Gate C role")
	socket := flags.String("socket", "", "owned Unix socket path")
	nonce := flags.String("nonce", "", "32-byte canonical hex nonce")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 {
		return 2
	}
	if err := siteexperiment.RunRole(context.Background(), *role, siteexperiment.RoleConfig{SocketPath: *socket, NonceHex: *nonce}); err != nil {
		fmt.Fprintf(os.Stderr, "reference-site-lab: %v\n", err)
		return 1
	}
	return 0
}
