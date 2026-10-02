//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// Tests share only an immutable executable, never mutable domain roots. Build
// once: repeated Go package scans on the Docker source bind mount are costly.
var commandBuild struct {
	once      sync.Once
	directory string
	path      string
	err       error
	output    []byte
}

func TestMain(m *testing.M) {
	code := m.Run()
	if commandBuild.directory != "" {
		if err := os.RemoveAll(commandBuild.directory); err != nil {
			fmt.Fprintln(os.Stderr, "test binary cleanup failed")
			code = 1
		}
	}
	os.Exit(code)
}
func compiledCommand(t *testing.T) string {
	t.Helper()
	commandBuild.once.Do(func() {
		commandBuild.directory, commandBuild.err = os.MkdirTemp("", "ardents-next-tests-")
		if commandBuild.err != nil {
			return
		}
		commandBuild.path = filepath.Join(commandBuild.directory, "ardents-next")
		cmd := exec.CommandContext(t.Context(), "go", "build", "-o", commandBuild.path, ".")
		commandBuild.output, commandBuild.err = cmd.CombinedOutput()
	})
	if commandBuild.err != nil {
		t.Fatalf("command build %v: %s", commandBuild.err, commandBuild.output)
	}
	return commandBuild.path
}
