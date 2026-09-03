//go:build ignore

// Command check-tools verifies that quality checks use the reviewed versions.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type toolRequirement struct {
	name    string
	version string
	args    []string
	module  string
}

var requiredTools = []toolRequirement{
	{name: "staticcheck", version: "2025.1.1", args: []string{"-version"}},
	{name: "govulncheck", version: "govulncheck@v1.1.4", args: []string{"-version"}},
	{name: "deadcode", version: "v0.48.0", module: "golang.org/x/tools"},
}

func main() {
	failed := false
	for _, tool := range requiredTools {
		output, err := toolVersion(tool)
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

func toolVersion(tool toolRequirement) ([]byte, error) {
	if tool.module == "" {
		return exec.Command(tool.name, tool.args...).CombinedOutput()
	}
	path, err := exec.LookPath(tool.name)
	if err != nil {
		return nil, err
	}
	output, err := exec.Command("go", "version", "-m", path).CombinedOutput()
	if err != nil {
		return nil, err
	}
	if !strings.Contains(string(output), "mod\t"+tool.module+"\t") {
		return output, fmt.Errorf("%s is not built from %s", tool.name, tool.module)
	}
	return output, nil
}
