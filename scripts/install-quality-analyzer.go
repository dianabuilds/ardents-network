//go:build ignore

// Command install-quality-analyzer builds one reviewed compiler-compatible
// analyzer in an external temporary module. Only make tools-install invokes it.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const staticcheckBuildModule = `module ardents-staticcheck-build

go 1.27.2

require honnef.co/go/tools v0.8.1

replace honnef.co/go/tools => github.com/stefanb/go-tools v0.7.0-0.dev.0.20261008215554-a0a7f6a7b6af
`

const staticcheckBuildSums = `github.com/stefanb/go-tools v0.7.0-0.dev.0.20261008215554-a0a7f6a7b6af h1:3rKlGcVFB2S4ULUqeV8rT12r7zFyZiWuuN8QTDTXOsg=
github.com/stefanb/go-tools v0.7.0-0.dev.0.20261008215554-a0a7f6a7b6af/go.mod h1:xG1Q4n+mg3bKZrO6RWFA9jFlkqplflmDRHGeuexiLDs=
`

const errcheckBuildModule = `module ardents-errcheck-build

go 1.27.2

require (
 github.com/kisielk/errcheck v1.20.0
 golang.org/x/tools v0.51.0
)
`

const errcheckBuildSums = `github.com/kisielk/errcheck v1.20.0 h1:9rwHBNKzd4wkDWcROy3DvFGNqEPlkxBg305rvk7HabI=
github.com/kisielk/errcheck v1.20.0/go.mod h1:O+f80MKNwX8Oor2jwgpeQ9An7uJm+hRSgT+h22knRJU=
golang.org/x/tools v0.51.0 h1:k4Xc/1Om9jwkBJBo4NVLMSARBoWtK10mx+W5BnXCeAI=
golang.org/x/tools v0.51.0/go.mod h1:9eEncMayCV6zRMGhR5eZEC2iBx98qWcF1HZ9Z7wJOoA=
`

func main() {
	tool := flag.String("tool", "staticcheck", "reviewed analyzer: staticcheck or errcheck")
	cache := flag.String("cache", "", "external quality-cache directory")
	output := flag.String("output", "", "reviewed tool output directory")
	flag.Parse()
	if err := installAnalyzer(*tool, *cache, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func installAnalyzer(tool, cache, output string) error {
	module, sums, program := staticcheckBuildModule, staticcheckBuildSums, "honnef.co/go/tools/cmd/staticcheck"
	switch tool {
	case "staticcheck":
	case "errcheck":
		module, sums, program = errcheckBuildModule, errcheckBuildSums, "github.com/kisielk/errcheck"
	default:
		return fmt.Errorf("unreviewed analyzer %q", tool)
	}
	if cache == "" || output == "" {
		return fmt.Errorf("cache and output directories are required")
	}
	cache, err := filepath.Abs(cache)
	if err != nil {
		return err
	}
	working, err := os.Getwd()
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(working, cache)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("tool build cache must be outside the repository")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cache, 0700); err != nil {
		return err
	}
	build, err := os.MkdirTemp(cache, tool+"-build-")
	if err != nil {
		return err
	}
	// Retain this exact build module alongside external receipts. No repository
	// file or authenticated module-cache member is modified.
	for name, body := range map[string]string{"go.mod": module, "go.sum": sums} {
		if err := os.WriteFile(filepath.Join(build, name), []byte(body), 0600); err != nil {
			return err
		}
	}
	fmt.Println(tool+" build module:", build)
	run := func(args ...string) error {
		command := exec.Command("go", args...)
		command.Dir = build
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
		}
		return nil
	}
	if err := run("mod", "download", "all"); err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	name := tool
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := run("build", "-mod=mod", "-trimpath", "-buildvcs=false", "-o", filepath.Join(output, name), program); err != nil {
		return err
	}
	if err := run("mod", "verify"); err != nil {
		return err
	}
	return run("version", "-m", filepath.Join(output, name))
}
