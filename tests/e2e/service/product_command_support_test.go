package service_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func buildProductCommand(t *testing.T, name string) string {
	t.Helper()
	prebuilt := os.Getenv("ARDENTS_E2E_PRODUCT_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
	if prebuilt != "" {
		info, err := os.Stat(prebuilt)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("prebuilt product command %q is not a regular file: %v", name, err)
		}
		return prebuilt
	}
	return buildCommand(t, name, "./cmd/"+name)
}

func buildCommand(t *testing.T, name, packagePath string) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	filename := name
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	path := filepath.Join(t.TempDir(), filename)
	arguments := []string{"build", "-trimpath", "-buildvcs=false"}
	arguments = append(arguments, "-o", path, packagePath)
	command := exec.Command("go", arguments...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, output)
	}
	return path
}
