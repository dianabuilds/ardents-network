package processfixture

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// CommandBuild shares an immutable Ardents CLI across one test package.
type CommandBuild struct {
	Repository string
	Name       string
	// ArtifactEnv selects a profile artifact ahead of ARDENTS_PREBUILT_NEXT.
	ArtifactEnv string

	once      sync.Once
	directory string
	path      string
	err       error
	output    []byte
}

// Binary returns the fixture's prebuilt CLI or builds it once from Repository.
func (build *CommandBuild) Binary(t testing.TB) string {
	t.Helper()
	prebuilt := os.Getenv(build.ArtifactEnv)
	if prebuilt == "" {
		prebuilt = os.Getenv("ARDENTS_PREBUILT_NEXT")
	}
	if prebuilt != "" {
		path, err := filepath.Abs(prebuilt)
		if err != nil {
			t.Fatalf("resolve prebuilt CLI: %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("inspect prebuilt CLI: %v", err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("prebuilt CLI %q is not a regular file", path)
		}
		return path
	}
	build.once.Do(func() {
		build.directory, build.err = os.MkdirTemp("", "ardents-next-tests-")
		if build.err != nil {
			return
		}
		name := build.Name
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		build.path = filepath.Join(build.directory, name)
		command := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", build.path, "./cmd/ardents-next")
		command.Dir = build.Repository
		command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
		build.output, build.err = command.CombinedOutput()
	})
	if build.err != nil {
		t.Fatalf("build CLI: %v: %s", build.err, build.output)
	}
	return build.path
}

// Run runs the package tests and removes the package-owned build directory.
func (build *CommandBuild) Run(m *testing.M) int {
	code := m.Run()
	if build.directory != "" {
		if err := os.RemoveAll(build.directory); err != nil {
			fmt.Fprintln(os.Stderr, "test binary cleanup failed:", err)
			return 1
		}
	}
	return code
}
