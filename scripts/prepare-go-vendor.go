//go:build ignore

// Command prepare-go-vendor creates a verified, disposable Docker build
// context outside the repository.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fatal(errors.New("usage: go run ./scripts/prepare-go-vendor.go ABSOLUTE_TEMP_DESTINATION"))
	}
	destination, err := filepath.Abs(os.Args[1])
	if err != nil || filepath.Clean(destination) != os.Args[1] {
		fatal(errors.New("vendor destination must be an absolute canonical path"))
	}
	temporaryRoot, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		fatal(err)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(destination))
	if err != nil {
		fatal(err)
	}
	relative, err := filepath.Rel(temporaryRoot, parent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		fatal(errors.New("vendor destination must be inside the system temporary directory"))
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		fatal(errors.New("vendor destination must not already exist"))
	}
	for _, arguments := range [][]string{{"mod", "download"}, {"mod", "verify"}, {"mod", "vendor", "-o", destination}} {
		command := exec.Command("go", arguments...)
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOENV=off")
		if err := command.Run(); err != nil {
			fatal(fmt.Errorf("go %s: %w", strings.Join(arguments, " "), err))
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "prepare-go-vendor:", err)
	os.Exit(1)
}
