package architecture

import (
	"bytes"
	"fmt"
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
	source := extractOwnedCandidate(t, root, "network", "application-interface-v1")
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
	extractedOne := extractOwnedCandidate(t, root, "network", "application-interface-v1")
	extractedTwo := extractOwnedCandidate(t, root, "network", "application-interface-v1")

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
	runExternal(t, repository, "make", "headless-build",
		"HEADLESS_ARTIFACT_ROOT="+headlessRoot,
		"CANONICAL_GO_BUILD_FLAGS=-trimpath")

	platform := runtime.GOOS + "-" + runtime.GOARCH
	result := make(map[string][]byte)
	for _, command := range strings.Fields(string(readProjectFile(t, repository, "tests/profiles/headless-commands.txt"))) {
		path := filepath.Join(headlessRoot, filepath.Base(command)+"-"+platform+executableSuffix())
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
	if len(result) != 4 {
		t.Fatalf("selected post-retirement artifact set has %d commands, want 4", len(result))
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

func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
