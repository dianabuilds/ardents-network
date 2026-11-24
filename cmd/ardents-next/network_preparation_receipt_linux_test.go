//go:build linux && network_preparation_native

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

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
			if code := runNetworkPreparation(t.Context(), []string{"prepare-initial-node-record", path, root}, out); code == 0 {
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

func TestNetworkPreparationReceiptInterruptsActualBlockedPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
		if err := w.Close(); err != nil {
			t.Error(err)
		}
	})
	fd := int(w.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	for {
		_, err := syscall.Write(fd, bytes.Repeat([]byte{1}, 4096))
		if errors.Is(err, syscall.EAGAIN) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- writeNetworkPreparationReceipt(ctx, w, []byte("public receipt\n")) }()
	select {
	case err := <-done:
		t.Fatal("full pipe completed before cancellation", err)
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked receipt did not join")
	}
}
