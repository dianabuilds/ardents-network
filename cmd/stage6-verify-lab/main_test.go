package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/lab/stage6evidence"
)

func TestRunPublishesPassAndRejectsMissingArguments(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(t.TempDir(), "stage6-evidence-lab")
	if os.PathSeparator == '\\' {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "../stage6-evidence-lab")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build evidence command: %v\n%s", err, output)
	}
	if err := stage6evidence.Run(root, "source", "clean", binary); err != nil {
		t.Fatal(err)
	}
	arguments := []string{"-manifest-root", filepath.Join(root, "manifest"), "-evidence-root", filepath.Join(root, "evidence"),
		"-private-root", filepath.Join(root, "private"), "-verdict-root", filepath.Join(root, "verdict")}
	if code := run(arguments); code != 0 {
		t.Fatalf("verify code=%d", code)
	}
	if code := run(nil); code != 2 {
		t.Fatalf("missing argument code=%d", code)
	}
}
