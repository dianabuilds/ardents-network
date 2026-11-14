//go:build linux

package reachability_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
)

func TestRetainedStoreMissingHistoryNeverCreatesOrRepairs(t *testing.T) {
	for _, missing := range []string{"root", ".ardents-reachability-store-v3", ".ardents-reachability-store-lock", "records", "stage"} {
		t.Run(missing, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "store")
			config := reachability.StoreConfig{Root: root, Network: [32]byte{1}}
			if missing == "root" {
				if store, err := reachability.OpenRetainedStore(config); err == nil {
					store.Close()
					t.Fatal("missing root accepted")
				}
				if _, err := os.Lstat(root); !os.IsNotExist(err) {
					t.Fatal("retained opening created root", err)
				}
				return
			}
			store, err := reachability.OpenStore(config)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			if missing == "stage" {
				stage := filepath.Join(root, "records", ".record-stage")
				raw := []byte("original uncertain unpublished write")
				if err := os.WriteFile(stage, raw, 0600); err != nil {
					t.Fatal(err)
				}
				if store, err := reachability.OpenRetainedStore(config); err == nil {
					store.Close()
					t.Fatal("uncertain stage adopted")
				}
				after, err := os.ReadFile(stage)
				if err != nil || !bytes.Equal(raw, after) {
					t.Fatal("retained opening repaired uncertain stage", err)
				}
			} else {
				path := filepath.Join(root, missing)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if store, err := reachability.OpenRetainedStore(config); err == nil {
					store.Close()
					t.Fatal("missing history accepted")
				}
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatal("retained opening recreated history", err)
				}
			}
		})
	}
}

func TestRetainedStorePreservesSignedFloorAndStaleRefusal(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	config := reachability.StoreConfig{Root: t.TempDir(), Network: network}
	store, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Publish(raw, profile, at, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	retained, err := reachability.OpenRetainedStore(config)
	if err != nil {
		t.Fatal(err)
	}
	got, outcome, readErr := retained.Lookup(target, profile, at)
	closeErr := retained.Close()
	if readErr != nil || closeErr != nil || outcome != reachability.Accepted || !bytes.Equal(raw, got) {
		t.Fatal("original signed floor differs", outcome, readErr, closeErr)
	}
}
