package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/lab/tooling"
)

func toolingVerify(arguments []string) int {
	flags := flag.NewFlagSet("tooling-verify", flag.ContinueOnError)
	lockPath := flags.String("lock", "", "absolute tool identity lock path")
	bundlePath := flags.String("bundle", "", "absolute external package bundle path")
	repositoryRoot := flags.String("repository-root", "", "absolute repository root")
	if err := flags.Parse(arguments); err != nil {
		return 64
	}
	lockSHA256, packageCount, baseImageID, sourceSHA256, err := tooling.VerifyInputs(*lockPath, *bundlePath, *repositoryRoot)
	if err != nil {
		return commandError("verify Carrier Lab tooling inputs", err)
	}
	fmt.Printf("Carrier Lab tool inputs: verified\nLock SHA-256: %s\nPackages: %d\nLocal base image ID: %s\nQualification source SHA-256: %s\n", lockSHA256, packageCount, baseImageID, sourceSHA256)
	return 0
}
func toolingRole(arguments []string) int {
	if len(arguments) < 3 || len(arguments) > 4 {
		fmt.Fprintln(os.Stderr, "usage: carrier-lab tooling-role <tracer|shape|capture> <run-id> <alpha|beta> [fault]")
		return 64
	}
	fault := ""
	if len(arguments) == 4 {
		fault = arguments[3]
	}
	if err := tooling.RunRole(arguments[0], arguments[1], arguments[2], fault); err != nil {
		return commandError("tooling role", err)
	}
	return 0
}

func pressureMemory(arguments []string) int {
	if err := tooling.RunMemoryPressure(arguments); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 64
	}
	return 0
}
