//go:build !linux && !windows

package service_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	pty "github.com/aymanbagabas/go-pty"
)

func TestCustodyUnsupportedTerminalRefusesWithoutRecord(t *testing.T) {
	binary := buildProductCommand(t, "ardents-custody")
	directory := t.TempDir()
	root := filepath.Join(directory, "vault")
	one, two, three := [32]byte{71}, [32]byte{72}, [32]byte{73}
	output, err := runInteractiveProductCommandResult(t, directory, binary, nil,
		"create-admission-authority", "-vault-root", root,
		"-environment-commitment", hex.EncodeToString(one[:]),
		"-network-commitment", hex.EncodeToString(two[:]),
		"-root-commitment", hex.EncodeToString(three[:]))
	if err == nil || !bytes.Contains(output, []byte("custody terminal reader requires Windows or Linux")) ||
		bytes.Contains(output, []byte("vault-create password:")) {
		t.Fatalf("unsupported terminal did not refuse before prompting: %v / %s", err, output)
	}
	entries, err := os.ReadDir(filepath.Join(root, "records"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("unsupported terminal created a record: %v / %v", entries, err)
	}
}

// No input is sent on an unsupported host. The real PTY only demonstrates
// that refusal comes from the platform reader, not from a missing terminal.
func interactiveProductLineEnding(bool) string                { return "\n" }
func waitForInteractiveNoEcho(context.Context, pty.Pty) error { return nil }
func isExpectedInteractiveProductReadError(err error) bool    { return errors.Is(err, syscall.EIO) }
