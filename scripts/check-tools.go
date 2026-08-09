//go:build ignore

// Command check-tools verifies that quality checks use the reviewed versions.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var requiredTools = []struct {
	name    string
	version string
	args    []string
}{
	{name: "staticcheck", version: "2025.1.1", args: []string{"-version"}},
	{name: "govulncheck", version: "govulncheck@v1.1.4", args: []string{"-version"}},
}

func main() {
	failed := false
	for _, tool := range requiredTools {
		output, err := exec.Command(tool.name, tool.args...).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s is missing or cannot run; use `make tools-install`: %v\n", tool.name, err)
			failed = true
			continue
		}
		if !strings.Contains(string(output), tool.version) {
			fmt.Fprintf(os.Stderr, "%s has the wrong version; want %s, got %q\n", tool.name, tool.version, strings.TrimSpace(string(output)))
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}
