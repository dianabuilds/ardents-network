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

func TestCanonicalCommandBuildIsRepositoryRepresentationIndependent(t *testing.T) {
	if os.Getenv("ARDENTS_CANONICAL_BUILD_REPRESENTATIONS") != "1" {
		return
	}
	root := repositoryRoot(t)
	source := extractOwnedCandidate(t, root, "network", "application-browser", "application-interface-v1")
	runProofGit(t, source, "init", "--quiet")
	runProofGit(t, source, "config", "user.name", "Ardents build proof")
	runProofGit(t, source, "config", "user.email", "build-proof@invalid.example")
	runProofGit(t, source, "add", ".")
	runProofGit(t, source, "commit", "--quiet", "-m", "build proof source")

	normalOne := cloneCandidate(t, source, "normal-clone-one")
	normalTwo := cloneCandidate(t, source, "normal-clone-two")
	linkedParent := t.TempDir()
	linked := filepath.Join(linkedParent, "linked-worktree")
	runProofGit(t, normalOne, "worktree", "add", "--quiet", "--detach", linked, "HEAD")
	extractedOne := extractOwnedCandidate(t, root, "network", "application-browser", "application-interface-v1")
	extractedTwo := extractOwnedCandidate(t, root, "network", "application-browser", "application-interface-v1")

	representations := []struct {
		name string
		root string
	}{
		{name: "normal clone 1", root: normalOne},
		{name: "normal clone 2", root: normalTwo},
		{name: "linked worktree", root: linked},
		{name: "VCS-free extraction 1", root: extractedOne},
		{name: "VCS-free extraction 2", root: extractedTwo},
	}
	built := make(map[string]map[string][]byte, len(representations))
	for _, representation := range representations {
		built[representation.name] = buildCanonicalArtifacts(t, representation.root)
	}
	for command, normalBytes := range built["normal clone 1"] {
		for _, representation := range representations[1:] {
			candidateBytes, ok := built[representation.name][command]
			if !ok {
				t.Errorf("%s omitted selected artifact %s", representation.name, command)
				continue
			}
			if !bytes.Equal(normalBytes, candidateBytes) {
				t.Errorf("selected artifact %s depends on %s repository representation", command, representation.name)
			}
		}
	}
}

func cloneCandidate(t *testing.T, source, name string) string {
	t.Helper()
	parent := t.TempDir()
	clone := filepath.Join(parent, name)
	runProofGit(t, parent, "clone", "--quiet", "--no-local", source, clone)
	return clone
}

func TestCanonicalBuildGitProofIgnoresAmbientPolicy(t *testing.T) {
	ambient := t.TempDir()
	hooks := filepath.Join(ambient, "hooks")
	template := filepath.Join(ambient, "template")
	if err := os.MkdirAll(hooks, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(template, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\nexit 93\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(template, "ambient-template-marker"), []byte("inherited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("[commit]\n\tgpgSign = true\n[core]\n\thooksPath = %s\n[init]\n\ttemplateDir = %s\n[protocol \"file\"]\n\tallow = never\n",
		filepath.ToSlash(hooks), filepath.ToSlash(template))
	configPath := filepath.Join(ambient, "ambient.gitconfig")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", configPath)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "0")

	source := t.TempDir()
	runProofGit(t, source, "init", "--quiet")
	runProofGit(t, source, "config", "user.name", "Ardents build proof")
	runProofGit(t, source, "config", "user.email", "build-proof@invalid.example")
	if err := os.WriteFile(filepath.Join(source, "source.txt"), []byte("source\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runProofGit(t, source, "add", ".")
	runProofGit(t, source, "commit", "--quiet", "-m", "isolated source")
	if _, err := os.Stat(filepath.Join(source, ".git", "ambient-template-marker")); !os.IsNotExist(err) {
		t.Fatalf("proof Git init inherited ambient template: %v", err)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	runProofGit(t, filepath.Dir(clone), "clone", "--quiet", "--no-local", source, clone)
}

func TestCanonicalCommandBuildPolicyCannotDrift(t *testing.T) {
	root := repositoryRoot(t)
	makefile := string(readProjectFile(t, root, "Makefile"))
	canonicalGoBuildFlags(t, root)
	for _, required := range []string{
		"override CANONICAL_GO_BUILD_FLAGS := -trimpath -buildvcs=false",
		"$(foreach command,$(HEADLESS_COMMANDS),go build $(CANONICAL_GO_BUILD_FLAGS)",
		"$(foreach command,$(BROWSER_COMMANDS),go build $(CANONICAL_GO_BUILD_FLAGS)",
		"HEADLESS_ARTIFACT_MKDIR = powershell -NoProfile -Command \"[System.IO.Directory]::CreateDirectory('$(HEADLESS_ARTIFACT_ROOT)') | Out-Null\"",
	} {
		if !strings.Contains(makefile, required) {
			t.Errorf("canonical Make build policy is missing %q", required)
		}
	}
}

func buildCanonicalArtifacts(t *testing.T, repository string) map[string][]byte {
	t.Helper()
	artifactParent := t.TempDir()
	headlessRoot := filepath.Join(artifactParent, "headless")
	browserRoot := filepath.Join(artifactParent, "browser")
	runExternal(t, repository, "make", "headless-build", "browser-build",
		"HEADLESS_ARTIFACT_ROOT="+headlessRoot, "BROWSER_ARTIFACT_ROOT="+browserRoot,
		"CANONICAL_GO_BUILD_FLAGS=-trimpath")

	platform := runtime.GOOS + "-" + runtime.GOARCH
	result := make(map[string][]byte)
	for _, inventory := range []struct {
		path string
		root string
	}{
		{path: "tests/profiles/headless-commands.txt", root: headlessRoot},
		{path: "tests/profiles/browser-commands.txt", root: browserRoot},
	} {
		for _, command := range strings.Fields(string(readProjectFile(t, repository, inventory.path))) {
			path := filepath.Join(inventory.root, filepath.Base(command)+"-"+platform+executableSuffix())
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read selected artifact %s: %v", command, err)
			}
			if _, duplicate := result[command]; duplicate {
				t.Fatalf("selected artifact command is duplicated: %s", command)
			}
			assertNoVCSBuildSettings(t, path)
			result[command] = raw
		}
	}
	if len(result) != 6 {
		t.Fatalf("selected post-retirement artifact set has %d commands, want 6", len(result))
	}
	return result
}

func assertNoVCSBuildSettings(t *testing.T, artifact string) {
	t.Helper()
	command := exec.Command("go", "version", "-m", artifact)
	command.Env = append(externalEnvironment(os.Environ()), "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("inspect build metadata for %s: %v\n%s", artifact, err, output)
	}
	var settings []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "\tvcs") {
			settings = append(settings, strings.TrimSpace(line))
		}
	}
	if len(settings) != 0 {
		t.Errorf("canonical artifact %s retained implicit VCS settings: %s", artifact, strings.Join(settings, ", "))
	}
}

func TestExtractionCommandsDoNotInheritParentRepositoryControls(t *testing.T) {
	environment := externalEnvironment([]string{
		"PATH=tool-path",
		"GIT_DIR=parent-git-dir",
		"GIT_WORK_TREE=parent-worktree",
		"GIT_COMMON_DIR=parent-common-dir",
		"GIT_CONFIG_COUNT=1",
		"MAKEFLAGS=--jobserver-auth=parent",
		"MFLAGS=-j4",
		"MAKELEVEL=2",
	})
	if strings.Join(environment, "\n") != "PATH=tool-path" {
		t.Fatalf("extraction command environment retained parent repository controls: %v", environment)
	}
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
	suffix := executableSuffix()
	adapter := filepath.Join(artifacts, "ardents-browser-"+platform+suffix)
	entry := filepath.Join(artifacts, "ardents-browser-entry-"+platform+suffix)
	buildCandidateCommand(t, root, candidate, "./cmd/ardents-browser", adapter)
	buildCandidateCommand(t, root, candidate, "./cmd/ardents-browser-entry", entry)
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
		buildCandidateCommand(t, root, candidate, command, filepath.Join(t.TempDir(), filepath.Base(command)))
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

func buildCandidateCommand(t *testing.T, policyRoot, candidate, command, output string) {
	t.Helper()
	arguments := append([]string{"build"}, canonicalGoBuildFlags(t, policyRoot)...)
	arguments = append(arguments, "-o", output, command)
	runCandidateGo(t, candidate, arguments...)
}

func canonicalGoBuildFlags(t *testing.T, candidate string) []string {
	t.Helper()
	const prefix = "override CANONICAL_GO_BUILD_FLAGS := "
	for _, line := range strings.Split(string(readProjectFile(t, candidate, "Makefile")), "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		flags := strings.Fields(strings.TrimPrefix(line, prefix))
		if strings.Join(flags, " ") != "-trimpath -buildvcs=false" {
			t.Fatalf("canonical Go build flags = %q, want explicit no-VCS stamping policy", flags)
		}
		return flags
	}
	t.Fatal("Makefile has no canonical Go build policy")
	return nil
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

func runProofGit(t *testing.T, directory string, arguments ...string) {
	t.Helper()
	emptyPolicyRoot := filepath.ToSlash(t.TempDir())
	policy := []string{
		"-c", "commit.gpgSign=false",
		"-c", "tag.gpgSign=false",
		"-c", "core.hooksPath=" + emptyPolicyRoot,
		"-c", "init.templateDir=" + emptyPolicyRoot,
		"-c", "protocol.file.allow=always",
		"-c", "core.autocrlf=false",
		"-c", "core.safecrlf=false",
	}
	command := exec.Command("git", append(policy, arguments...)...)
	command.Dir = directory
	command.Env = append(externalEnvironment(os.Environ()),
		"GOWORK=off",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
}

func externalEnvironment(input []string) []string {
	gitRepositoryVariables := map[string]bool{
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
		"GIT_COMMON_DIR":                   true,
		"GIT_CONFIG":                       true,
		"GIT_CONFIG_COUNT":                 true,
		"GIT_CONFIG_GLOBAL":                true,
		"GIT_CONFIG_NOSYSTEM":              true,
		"GIT_CONFIG_PARAMETERS":            true,
		"GIT_CONFIG_SYSTEM":                true,
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
		"GIT_TEMPLATE_DIR":                 true,
		"GIT_TERMINAL_PROMPT":              true,
		"GIT_WORK_TREE":                    true,
		"GIT_ALLOW_PROTOCOL":               true,
		"GIT_PROTOCOL_FROM_USER":           true,
		"MAKEFLAGS":                        true,
		"MAKELEVEL":                        true,
		"MFLAGS":                           true,
	}
	result := make([]string, 0, len(input))
	for _, value := range input {
		name, _, found := strings.Cut(value, "=")
		if !found || gitRepositoryVariables[strings.ToUpper(name)] {
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

func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
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
