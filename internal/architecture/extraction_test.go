package architecture

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOwnedExtractionExcludesRepositoryMetadata(t *testing.T) {
	candidate := extractOwnedCandidate(t, repositoryRoot(t), "network", "application-interface-v1")
	if _, err := os.Lstat(filepath.Join(candidate, ".git")); !os.IsNotExist(err) {
		t.Fatalf("owned extraction retained repository metadata: %v", err)
	}
}

func TestExtractionCommandsDoNotInheritParentRepositoryOrMakeControls(t *testing.T) {
	environment := externalEnvironment([]string{
		"PATH=tool-path",
		"GIT_DIR=parent-git-dir",
		"GIT_WORK_TREE=parent-worktree",
		"GIT_COMMON_DIR=parent-common-dir",
		"GIT_CONFIG_COUNT=1",
		"MAKEFLAGS=--output-sync=target -j4",
		"MAKELEVEL=2",
		"MFLAGS=-j4",
		"GNUMAKEFLAGS=--warn-undefined-variables",
		"SHELL=/bin/sh",
	})
	if strings.Join(environment, "\n") != "PATH=tool-path" {
		t.Fatalf("extraction command environment retained parent repository or Make controls: %v", environment)
	}
}

func TestCanonicalNetworkCommandBuildIsRepositoryRepresentationIndependent(t *testing.T) {
	root := repositoryRoot(t)
	plain := extractOwnedCandidate(t, root, "network", "application-interface-v1")
	versioned := extractOwnedCandidate(t, root, "network", "application-interface-v1")
	runExternal(t, versioned, "git", "init", "--quiet")
	runExternal(t, versioned, "git", "config", "user.name", "Ardents build proof")
	runExternal(t, versioned, "git", "config", "user.email", "build-proof@invalid.example")
	runExternal(t, versioned, "git", "add", ".")
	runExternal(t, versioned, "git", "commit", "--quiet", "-m", "build proof source")

	linkedArtifacts := buildCanonicalNetworkArtifactSet(t, root)
	plainArtifacts := buildCanonicalNetworkArtifactSet(t, plain)
	versionedArtifacts := buildCanonicalNetworkArtifactSet(t, versioned)
	for name, linkedBytes := range linkedArtifacts {
		plainBytes, plainOK := plainArtifacts[name]
		versionedBytes, versionedOK := versionedArtifacts[name]
		if !plainOK || !versionedOK || !bytes.Equal(linkedBytes, plainBytes) || !bytes.Equal(plainBytes, versionedBytes) {
			t.Fatalf("canonical Network artifact %s depends on linked, extracted, or normal Git repository representation", name)
		}
	}
}

func buildCanonicalNetworkArtifactSet(t *testing.T, candidate string) map[string][]byte {
	t.Helper()
	root := t.TempDir()
	headlessRoot := filepath.Join(root, "headless")
	ceremonyRoot := filepath.Join(root, "ceremony")
	runExternal(t, candidate, "make", "HEADLESS_ARTIFACT_ROOT="+shellPath(headlessRoot),
		"CEREMONY_ARTIFACT_ROOT="+shellPath(ceremonyRoot), "headless-build", "ceremony-build")
	platform := runtime.GOOS + "-" + runtime.GOARCH
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	artifacts := make(map[string][]byte)
	readArtifacts := func(profile, artifactRoot string) {
		for _, command := range strings.Fields(string(readProjectFile(t, candidate, profile))) {
			name := filepath.Base(command) + "-" + platform + suffix
			content, err := os.ReadFile(filepath.Join(artifactRoot, name))
			if err != nil {
				t.Fatalf("read canonical Network artifact %s: %v", name, err)
			}
			artifacts[name] = content
		}
	}
	readArtifacts("tests/profiles/headless-commands.txt", headlessRoot)
	readArtifacts("tests/profiles/local-ceremony-commands.txt", ceremonyRoot)
	return artifacts
}

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
		buildCanonicalNetworkCommand(t, candidate, command, filepath.Join(t.TempDir(), filepath.Base(command)))
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
		if relative == "go.mod" || relative == "go.sum" {
			return
		}
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

func buildCanonicalNetworkCommand(t *testing.T, candidate, command, output string) {
	t.Helper()
	runCandidateGo(t, candidate, "build", "-trimpath", "-buildvcs=false", "-o", output, command)
}

func runExternal(t *testing.T, directory, name string, arguments ...string) {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Env = append(externalEnvironment(os.Environ()), "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), err, output)
	}
}

func externalEnvironment(input []string) []string {
	parentControlVariables := map[string]bool{
		"GNUMAKEFLAGS":                     true,
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
		"GIT_COMMON_DIR":                   true,
		"GIT_CONFIG":                       true,
		"GIT_CONFIG_COUNT":                 true,
		"GIT_CONFIG_PARAMETERS":            true,
		"GIT_DIR":                          true,
		"GIT_GRAFT_FILE":                   true,
		"GIT_IMPLICIT_WORK_TREE":           true,
		"GIT_INDEX_FILE":                   true,
		"GIT_INTERNAL_SUPER_PREFIX":        true,
		"GIT_NO_REPLACE_OBJECTS":           true,
		"GIT_OBJECT_DIRECTORY":             true,
		"GIT_PREFIX":                       true,
		"GIT_REPLACE_REF_BASE":             true,
		"GIT_SHALLOW_FILE":                 true,
		"GIT_WORK_TREE":                    true,
		"MAKEFLAGS":                        true,
		"MAKELEVEL":                        true,
		"MFLAGS":                           true,
		"SHELL":                            true,
	}
	result := make([]string, 0, len(input))
	for _, value := range input {
		name, _, found := strings.Cut(value, "=")
		if !found || parentControlVariables[name] {
			continue
		}
		result = append(result, value)
	}
	return result
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
