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
	name      string
	version   string
	args      []string
	module    string
	goVersion string
}

var requiredTools = []toolRequirement{
	{name: "staticcheck", version: "2025.1.1", args: []string{"-version"}, goVersion: "go1.26.8"},
	{name: "govulncheck", version: "govulncheck@v1.1.4", args: []string{"-version"}, goVersion: "go1.26.8"},
	{name: "deadcode", version: "v0.48.0", module: "golang.org/x/tools", goVersion: "go1.26.8"},
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
		if err := toolBuildToolchain(tool); err != nil {
			fmt.Fprintf(os.Stderr, "%s was not built with the required Go toolchain: %v\n", tool.name, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func toolBuildToolchain(tool toolRequirement) error {
	path, err := exec.LookPath(tool.name)
	if err != nil {
		return err
	}
	output, err := exec.Command("go", "version", "-m", path).CombinedOutput()
	if err != nil {
		return err
	}
	firstLine := strings.SplitN(string(output), "\n", 2)[0]
	separator := strings.LastIndex(firstLine, ": ")
	if separator < 0 {
		return fmt.Errorf("cannot read compiler version from %q", strings.TrimSpace(firstLine))
	}
	if got := strings.TrimSpace(firstLine[separator+2:]); got != tool.goVersion {
		return fmt.Errorf("want %s, got %q", tool.goVersion, got)
	}
	return nil
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
