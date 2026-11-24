//go:build linux && network_preparation_native

package node_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNetworkPreparationNativeInputAndReceiptRefusals(t *testing.T) {
	binary := buildNodeCommand(t)
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(parent, "plan")
	key := bytes.Repeat([]byte{7}, 32)
	raw := preparationJSON(t, preparationRecordPlan(key))
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"symlink", "hardlink", "oversize", "malformed", "output failure"} {
		t.Run(fault, func(t *testing.T) {
			path := input
			out := io.Writer(io.Discard)
			if fault == "symlink" {
				path = filepath.Join(parent, "alias")
				if err := os.Symlink(input, path); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "hardlink" {
				path = filepath.Join(parent, "hardlink")
				if err := os.Link(input, path); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Remove(path); err != nil {
						t.Error(err)
					}
				})
			}
			if fault == "oversize" {
				path = filepath.Join(parent, "large")
				if err := os.WriteFile(path, bytes.Repeat([]byte(" "), (8<<20)+1), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "malformed" {
				path = filepath.Join(parent, "bad")
				if err := os.WriteFile(path, append(bytes.Clone(raw), raw...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "output failure" {
				full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = full.Close() })
				out = full
			}
			root := filepath.Join(parent, "output-"+fault)
			command := exec.CommandContext(t.Context(), binary, "network", "prepare-initial-node-record", path, root)
			command.Stdout = out
			if command.Run() == nil {
				t.Fatal("false successful receipt")
			}
			if fault != "output failure" {
				if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("invalid plan caused output effects", err)
				}
			} else if _, err := os.Stat(root); err != nil {
				t.Fatalf("producer did not reach receipt: %v", err)
			}
		})
	}
}
