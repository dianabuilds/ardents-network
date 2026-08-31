package architecture

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestApplicationExtractionRehearsal(t *testing.T) {
	if os.Getenv("ARDENTS_EXTRACTION_OWNER") != "application-browser" {
		return
	}
	root := repositoryRoot(t)
	candidate := extractOwnedCandidate(t, root, "application-browser", "application-interface-v1")
	runCandidateGo(t, candidate, "test", "./internal/browser/...", "./internal/application/interfacev1/...", "./cmd/ardents-browser", "./cmd/ardents-browser-entry")
	artifacts := filepath.Join(t.TempDir(), "browser-artifacts")
	if err := os.Mkdir(artifacts, 0o700); err != nil {
		t.Fatal(err)
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	adapter := filepath.Join(artifacts, "ardents-browser-"+platform+suffix)
	entry := filepath.Join(artifacts, "ardents-browser-entry-"+platform+suffix)
	buildCandidateCommand(t, candidate, "./cmd/ardents-browser", adapter)
	buildCandidateCommand(t, candidate, "./cmd/ardents-browser-entry", entry)
	shell := os.Getenv("ARDENTS_EXTRACTION_SHELL")
	if shell == "" {
		t.Fatal("Application extraction requires ARDENTS_EXTRACTION_SHELL")
	}
	runExternal(t, candidate, shell, shellPath(filepath.Join(candidate, "packaging", "browser-bundle", "test.sh")), platform, shellPath(adapter), shellPath(entry))
}

func TestNetworkExtractionRehearsal(t *testing.T) {
	if os.Getenv("ARDENTS_EXTRACTION_OWNER") != "network" {
		return
	}
	root := repositoryRoot(t)
	candidate := extractOwnedCandidate(t, root, "network", "application-interface-v1")
	for _, relative := range []string{"internal/browser", "cmd/ardents-browser", "cmd/ardents-browser-entry", "packaging/browser-bundle"} {
		if _, err := os.Stat(filepath.Join(candidate, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Fatalf("Network extraction retained Application implementation %s", relative)
		}
	}
	runCandidateGo(t, candidate, "test", "./cmd/ardents", "./cmd/ardents-node", "./cmd/ardents-control", "./cmd/ardents-custody", "./internal/application/broker", "./internal/application/interfacev1/...")
	for _, command := range strings.Fields(string(readProjectFile(t, candidate, "tests/profiles/headless-commands.txt"))) {
		buildCandidateCommand(t, candidate, command, filepath.Join(t.TempDir(), filepath.Base(command)))
	}
}

func extractOwnedCandidate(t *testing.T, root string, owners ...string) string {
	t.Helper()
	candidate := t.TempDir()
	registry := readOwnershipRegistry(t, root)
	wanted := make(map[string]bool, len(owners))
	for _, owner := range owners {
		wanted[owner] = true
	}
	for _, relative := range []string{"go.mod", "go.sum"} {
		copyCandidateFile(t, root, candidate, relative)
	}
	walk(t, root, func(path string, entry os.DirEntry) {
		if entry.IsDir() {
			return
		}
		relative := relativePath(t, root, path)
		for _, rule := range registry.Rules {
			if wanted[rule.Owner] && ruleMatches(rule, relative) {
				copyCandidateFile(t, root, candidate, relative)
				return
			}
		}
	})
	return candidate
}

func copyCandidateFile(t *testing.T, sourceRoot, targetRoot, relative string) {
	t.Helper()
	source := filepath.Join(sourceRoot, filepath.FromSlash(relative))
	target := filepath.Join(targetRoot, filepath.FromSlash(relative))
	info, err := os.Stat(source)
	if err != nil {
		t.Fatalf("inspect extraction source %s: %v", relative, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("create extraction directory for %s: %v", relative, err)
	}
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("copy extraction file %s: %v", relative, errorsJoin(copyErr, closeErr))
	}
}

func runCandidateGo(t *testing.T, candidate string, arguments ...string) {
	t.Helper()
	runExternal(t, candidate, "go", arguments...)
}

func buildCandidateCommand(t *testing.T, candidate, command, output string) {
	t.Helper()
	runCandidateGo(t, candidate, "build", "-trimpath", "-o", output, command)
}

func runExternal(t *testing.T, directory, name string, arguments ...string) {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), err, output)
	}
}

func shellPath(path string) string {
	if runtime.GOOS == "windows" {
		return filepath.ToSlash(path)
	}
	return path
}

func errorsJoin(first, second error) error {
	if first != nil && second != nil {
		return fmt.Errorf("%v; %w", first, second)
	}
	if first != nil {
		return first
	}
	return second
}
