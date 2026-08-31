//go:build ignore

// Command enrollment-artifact-name exposes the enrollment package's canonical
// platform artifact identity to repository-owned packaging scripts.
package main

import (
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: enrollment-artifact-name <command> <platform>")
		os.Exit(2)
	}
	fmt.Println(enrollment.ExecutableArtifactName(os.Args[1], os.Args[2]))
}
