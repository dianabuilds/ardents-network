package state_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func buildCommand(t *testing.T, name string) string {
	t.Helper()
	if root := os.Getenv("ARDENTS_E2E_COMMAND_ROOT"); root != "" {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root {
			t.Fatal("prebuilt E2E command root must be one clean absolute path")
		}
		path := filepath.Join(root, name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("prebuilt E2E command %s is not a regular executable", name)
		}
		return path
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	path := filepath.Join(t.TempDir(), name+suffix)
	command := exec.Command("go", "build", "-o", path, "./cmd/"+name)
	command.Dir = filepath.Join("..", "..", "..")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, output)
	}
	return path
}

func TestBuildCommandUsesDeclaredPrebuiltExecutable(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ardents-node")
	if err := os.WriteFile(path, []byte("exact prebuilt test command\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARDENTS_E2E_COMMAND_ROOT", root)
	if observed := buildCommand(t, "ardents-node"); observed != path {
		t.Fatalf("prebuilt command = %q, want %q", observed, path)
	}
}
