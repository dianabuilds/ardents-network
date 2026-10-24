//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"syscall"
	"testing"
)

// Native file primitives only: this fixture has four records and no account,
// generation, Release, manager or installed Source. It cannot satisfy the full
// initial115 grammar or grant runtime/continuation authority.
func nativeSourceInitialPending(t *testing.T, ctx context.Context, recoverable bool) (*sourceBirths, *sourceBirth, os.FileInfo, string) {
	t.Helper()
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid environment: isolated native file stand required", err)
	}
	base, err := os.MkdirTemp(os.Getenv("ARDENTS_SOURCE_STOPPED_FUTURE_PARENT"), "initial-pending-")
	if err != nil {
		t.Fatal(err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(base, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: actual ext4 required", err)
	}
	parent, err := os.Lstat(base)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "installation")
	path, journal := sourceInitialPendingPath(root), root+".source-initial-journal"
	var births *sourceBirths
	if recoverable {
		births, err = openSourceRecoverableInitialBirths(ctx, journal, map[string]bool{path: true})
	} else {
		births, err = openSourceBirths(ctx, journal, map[string]bool{path: true})
	}
	if err != nil {
		t.Fatal(errors.Join(err, births.close()))
	}
	t.Cleanup(func() { _ = births.close() })
	if err := births.write(path, sourceInitialPending(strings.Repeat("a", 64)), 0600, 0, 0, ctx.Err); err != nil {
		t.Fatal(err)
	}
	t.Log("retained native initial pending fixture", base)
	return births, births.files[0], parent, root
}

func TestSourceRecoveryInitialPendingNativePublicationAfterClose(t *testing.T) {
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	births, pending, parent, root := nativeSourceInitialPending(t, t.Context(), true)
	before := bytes.Clone(births.journal.body)
	if err := observeSourceInitialPending(root, nil); !errors.Is(err, ErrSourceRepairRequired) {
		t.Fatal("ordinary consumer bypassed the original pending marker", err)
	}
	if err := observeSourceInitialPending(root, births); err != nil {
		t.Fatal("original writer could not read its own native marker", err)
	}
	if err := publishSourceInitialPending(t.Context(), births, pending, parent); err == nil {
		t.Fatal("physical publication preceded original writer Close")
	}
	if err := births.close(); err != nil {
		t.Fatal(err)
	}
	if err := observeSourceInitialPending(root, nil); !errors.Is(err, ErrSourceRepairRequired) {
		t.Fatal("closed-but-unpublished marker granted completion", err)
	}
	prefix, err := openSourceRecoveryPrefix(t.Context(), births.journal.path, "initial")
	if err != nil || len(prefix.records) != 4 || prefix.records[0].Phase != "initial-recoverable" {
		_ = prefix.close()
		t.Fatal("new partial format lost its independent native birth observations", err)
	}
	if err := prefix.close(); err != nil {
		t.Fatal(err)
	}
	// The genuine producer separately supplies full admission and verification;
	// this is only its last no-further-I/O namespace publication primitive.
	if err := publishSourceInitialPending(t.Context(), births, pending, parent); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(pending.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("joined physical publication retained marker", err)
	}
	after, err := os.ReadFile(births.journal.path)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatal("marker publication rewrote original journal", err)
	}
	if actual := sourceHistoryDescriptors(t, map[string]bool{births.journal.path: true, pending.path: true, filepath.Dir(pending.path): true}); !reflect.DeepEqual(actual, map[string]int{}) {
		t.Fatal("original native descriptors survived Close/publication before GC", actual)
	}
}

func TestSourceRecoveryInitialPendingNativeFailureRetainsGuard(t *testing.T) {
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	for _, fault := range []string{"caller-canceled", "close-error", "guard-substitution", "parent-substitution", "legacy-writer"} {
		t.Run(fault, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			births, pending, parent, _ := nativeSourceInitialPending(t, ctx, fault != "legacy-writer")
			original := bytes.Clone(pending.body)
			if fault == "close-error" {
				if err := pending.file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			closed := births.close()
			if fault == "close-error" && !errors.Is(closed, os.ErrClosed) || fault != "close-error" && closed != nil {
				t.Fatal("actual original native Close result", closed)
			}
			switch fault {
			case "caller-canceled":
				cancel()
			case "guard-substitution":
				if err := os.Rename(pending.path, pending.path+".retained"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(pending.path, original, 0600); err != nil {
					t.Fatal(err)
				}
			case "parent-substitution":
				base := filepath.Dir(pending.path)
				if err := os.Rename(base, base+".retained"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(base, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(pending.path, original, 0600); err != nil {
					t.Fatal(err)
				}
			}
			result := publishSourceInitialPending(ctx, births, pending, parent)
			if result == nil || fault == "close-error" && !errors.Is(result, os.ErrClosed) || fault == "caller-canceled" && !errors.Is(result, context.Canceled) {
				t.Fatal("uncertain/foreign marker was published or lost original failure", result)
			}
			actual, err := os.ReadFile(pending.path)
			if err != nil || !bytes.Equal(actual, original) {
				t.Fatal("refusal deleted or changed retained marker", err)
			}
		})
	}
}
