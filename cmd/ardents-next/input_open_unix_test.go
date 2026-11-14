//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestInputOpenDoesNotWaitForSubstitutedFIFO(t *testing.T) {
	// Exercise the opening primitive in its own joined process. Regressing to
	// blocking Open must fail this test without stranding a test goroutine.
	if len(os.Args) >= 3 && os.Args[len(os.Args)-2] == "input-open-child" {
		file, err := openInputFile(os.Args[len(os.Args)-1])
		if err != nil {
			t.Fatal(err)
		}
		info, statErr := file.Stat()
		closeErr := file.Close()
		if statErr != nil || closeErr != nil || info.Mode()&os.ModeNamedPipe == 0 {
			t.Fatal("substituted FIFO was not observed and closed", statErr, closeErr)
		}
		return
	}
	root := t.TempDir()
	regular, fifo, selected := filepath.Join(root, "regular"), filepath.Join(root, "fifo"), filepath.Join(root, "selected")
	if err := os.WriteFile(regular, []byte("bounded"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(regular, selected); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(selected); err != nil || !info.Mode().IsRegular() {
		t.Fatal("regular precheck", err)
	}
	replacement := filepath.Join(root, "replacement")
	if err := os.Symlink(fifo, replacement); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, selected); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInputOpenDoesNotWaitForSubstitutedFIFO$", "--", "input-open-child", selected)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("FIFO open did not return and join: %v %s", err, output)
	}
	if _, err := readBounded(selected, 32); err == nil {
		t.Fatal("FIFO accepted as a bounded file")
	}
	if _, err := readBounded(os.DevNull, 32); err == nil {
		t.Fatal("device accepted as a bounded file")
	}
}
