//go:build ignore

// Command check-tools verifies that quality checks use the reviewed versions.
package main

import (
	"flag"
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
	{name: "staticcheck", version: "v0.8.1", module: "honnef.co/go/tools", goVersion: "go1.27.2"},
	{name: "govulncheck", version: "govulncheck@v1.8.0", args: []string{"-version"}, goVersion: "go1.27.2"},
	{name: "deadcode", version: "v0.50.0", module: "golang.org/x/tools", goVersion: "go1.27.2"},
}

func main() {
	diagnostic := flag.Bool("diagnostic", false, "also check the selected diagnostic analyzers")
	flag.Parse()
	tools := requiredTools
	if *diagnostic {
		tools = append(tools, toolRequirement{name: "errcheck", version: "v1.20.0", module: "github.com/kisielk/errcheck", goVersion: "go1.27.2"})
	}
	failed := false
	for _, tool := range tools {
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
			fmt.Fprintf(os.Stderr, "%s does not match the reviewed tool build: %v\n", tool.name, err)
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
	if tool.name == "staticcheck" {
		for _, required := range []string{
			"=>\tgithub.com/stefanb/go-tools\tv0.7.0-0.dev.0.20261008215554-a0a7f6a7b6af\th1:3rKlGcVFB2S4ULUqeV8rT12r7zFyZiWuuN8QTDTXOsg=",
			"dep\tgolang.org/x/tools\tv0.51.0\th1:k4Xc/1Om9jwkBJBo4NVLMSARBoWtK10mx+W5BnXCeAI=",
		} {
			if !strings.Contains(string(output), required) {
				return fmt.Errorf("missing reviewed Staticcheck build dependency %q; use make tools-install", required)
			}
		}
	}
	if tool.name == "errcheck" {
		for _, required := range []string{
			"mod\tgithub.com/kisielk/errcheck\tv1.20.0\th1:9rwHBNKzd4wkDWcROy3DvFGNqEPlkxBg305rvk7HabI=",
			"dep\tgolang.org/x/tools\tv0.51.0\th1:k4Xc/1Om9jwkBJBo4NVLMSARBoWtK10mx+W5BnXCeAI=",
		} {
			if !strings.Contains(string(output), required) {
				return fmt.Errorf("missing reviewed errcheck build dependency %q; use make tools-install DIAGNOSTIC_TOOLS=1", required)
			}
		}
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
