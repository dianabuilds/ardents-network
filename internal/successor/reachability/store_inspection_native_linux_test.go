//go:build linux && network_signing_native

package reachability_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
)

func descriptorInspectionFixture(t *testing.T) (reachability.StoreConfig, []byte, [32]byte, [32]byte, time.Time) {
	t.Helper()
	if os.Geteuid() != 0 || os.Getenv("ARDENTS_DESCRIPTOR_STORE_INSPECTION_NATIVE") != "1" {
		t.Fatal("invalid environment: explicit root Store inspection stand required")
	}
	parent, err := os.MkdirTemp("/var/lib", "d548-store-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	raw, target, network, profile, at := signedDescriptor(3)
	config := reachability.StoreConfig{Root: filepath.Join(parent, "store"), Network: network}
	store, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	_, publishErr := store.Publish(raw, profile, at, func() error { return nil })
	if err := errors.Join(publishErr, store.Close()); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(config.Root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, 988, 988)
	}); err != nil {
		t.Fatal(err)
	}
	return config, raw, target, profile, at
}

func TestDescriptorStoreInspectionNativeRetainsSignedHistoryAndExclusiveLease(t *testing.T) {
	config, raw, target, profile, at := descriptorInspectionFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
	defer cancel()
	h, err := reachability.OpenStoreInspection(ctx, config, 988, 988)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.Check(); err != nil {
		t.Fatal(err)
	}
	if other, err := reachability.OpenRetainedStore(config); err == nil {
		other.Close()
		t.Fatal("inspection lent or released its original exclusive lease")
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := reachability.OpenRetainedStore(config)
	if err != nil {
		t.Fatal(err)
	}
	got, outcome, readErr := store.Lookup(target, profile, at)
	if err := errors.Join(readErr, store.Close()); err != nil || outcome != reachability.Accepted || !bytes.Equal(got, raw) {
		t.Fatal("read-only inspection changed retained proof/floor", outcome, err)
	}
}

func TestDescriptorStoreInspectionNativeRefusesMissingUncertainAndForeignHistory(t *testing.T) {
	for _, changed := range []string{"root", ".ardents-reachability-store-v3", ".ardents-reachability-store-lock", "records", "stage", "foreign", "wrong-principal"} {
		t.Run(changed, func(t *testing.T) {
			config, _, _, _, _ := descriptorInspectionFixture(t)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
			defer cancel()
			path := filepath.Join(config.Root, changed)
			switch changed {
			case "root":
				config.Root += "-missing"
				path = config.Root
			case "stage":
				path = filepath.Join(config.Root, "records", ".record-stage")
				if err := os.WriteFile(path, []byte("original uncertain stage"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chown(path, 988, 988); err != nil {
					t.Fatal(err)
				}
			case "foreign":
				if err := os.WriteFile(path, []byte("foreign original"), 0600); err != nil {
					t.Fatal(err)
				}
			case "wrong-principal":
				if err := os.Chown(config.Root, 987, 987); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.RemoveAll(path); err != nil {
					t.Fatal(err)
				}
			}
			if h, err := reachability.OpenStoreInspection(ctx, config, 988, 988); err == nil {
				h.Close()
				t.Fatal("missing/uncertain/foreign retained history accepted")
			}
			if changed == "stage" {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != "original uncertain stage" {
					t.Fatal("inspection repaired uncertain stage", err)
				}
			} else if changed != "foreign" && changed != "wrong-principal" {
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatal("inspection recreated missing history", err)
				}
			}
		})
	}
}

func TestDescriptorStoreInspectionNativeRestorationCannotReviveFailedOriginal(t *testing.T) {
	config, _, _, _, _ := descriptorInspectionFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
	defer cancel()
	h, err := reachability.OpenStoreInspection(ctx, config, 988, 988)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	path := filepath.Join(config.Root, ".ardents-reachability-store-v3")
	retained := filepath.Join(filepath.Dir(config.Root), "retained-original-marker")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(path, 988, 988); err != nil {
		t.Fatal(err)
	}
	cause := h.Check()
	if cause == nil {
		t.Fatal("same-byte foreign inode accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(retained, path); err != nil {
		t.Fatal(err)
	}
	if err := h.Check(); !errors.Is(err, cause) {
		t.Fatal("restored path revived original inspection", err)
	}
	copyHandle := h
	first := h.Close()
	if !errors.Is(first, cause) || copyHandle.Close() != first {
		t.Fatal("copied Close lost original retained failure", first)
	}
}
