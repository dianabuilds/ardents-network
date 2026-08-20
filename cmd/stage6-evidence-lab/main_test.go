package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestCommandRunsEveryCellInAnIsolatedWorker(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "stage6-evidence-lab"+executableSuffix())
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build command: %v\n%s", err, output)
	}
	root := t.TempDir()
	command := exec.Command(binary, "-root", root, "-source-commit", "source", "-dirty-digest", "clean")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run command: %v\n%s", err, output)
	}
	launcherPID := int64(command.Process.Pid)
	for ordinal := range 27 {
		path := filepath.Join(root, "evidence", "cells", twoDigitOrdinal(ordinal), "terminal.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read terminal %d: %v", ordinal, err)
		}
		var terminal struct {
			WorkerPID int64 `json:"worker_pid"`
		}
		if err := json.Unmarshal(raw, &terminal); err != nil {
			t.Fatalf("decode terminal %d: %v", ordinal, err)
		}
		if terminal.WorkerPID <= 0 || terminal.WorkerPID == launcherPID {
			t.Fatalf("cell %d worker pid=%d launcher=%d", ordinal, terminal.WorkerPID, launcherPID)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "manifest" && entry.Name() != "evidence" && entry.Name() != "private" {
			t.Fatalf("launcher retained unexpected root %q", entry.Name())
		}
	}
}

func TestCommandRejectsIncompleteInput(t *testing.T) {
	if code := run([]string{"-root", t.TempDir()}); code != 1 {
		t.Fatalf("incomplete source code=%d", code)
	}
	if code := run(nil); code != 2 {
		t.Fatalf("missing root code=%d", code)
	}
}

func executableSuffix() string {
	if os.PathSeparator == '\\' {
		return ".exe"
	}
	return ""
}

func twoDigitOrdinal(value int) string {
	return strconv.Itoa(value/10) + strconv.Itoa(value%10)
}
