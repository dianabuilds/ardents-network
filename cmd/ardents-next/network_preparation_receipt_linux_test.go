//go:build linux && network_preparation_native

package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/node/provisioning"
)

func TestNetworkPreparationNativeInputAndReceiptRefusals(t *testing.T) {
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
	for _, fault := range []string{"symlink", "hardlink", "oversize", "malformed", "output failure", "short receipt"} {
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
				if err := os.WriteFile(path, bytes.Repeat([]byte(" "), provisioning.MaximumNetworkPreparationPlan+1), 0600); err != nil {
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
				out = preparationFailedWriter{}
			}
			if fault == "short receipt" {
				out = preparationShortWriter{}
			}
			root := filepath.Join(parent, "output-"+fault)
			if code := runNetwork(t.Context(), []string{"prepare-initial-node-record", path, root}, out); code == 0 {
				t.Fatal("false successful receipt")
			}
			if fault != "output failure" && fault != "short receipt" {
				if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("invalid plan caused output effects", err)
				}
			}
		})
	}
}

type preparationFailedWriter struct{}

func (preparationFailedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type preparationShortWriter struct{}

func (preparationShortWriter) Write([]byte) (int, error) { return 0, nil }
