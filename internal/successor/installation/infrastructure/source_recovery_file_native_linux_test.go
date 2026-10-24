//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"os"
	"runtime/debug"
	"strings"
	"testing"
)

// File-only causal cut after an actual synced empty birth. This is not an
// account/Release/manager/Source fixture and cannot establish full recovery.
func nativeSourceRecoveryEmptyBirth(t *testing.T) (*sourceRecoveryPrefix, string, []byte, error) {
	t.Helper()
	original, _, _, root := nativeSourceInitialPending(t, t.Context(), true)
	path := root + ".public-file"
	original.allowed[path] = true
	body := []byte("exact intended public artifact bytes")
	cut := errors.New("original empty birth cut before byte delivery")
	err := original.write(path, body, 0444, 0, 0, func() error {
		if original.sequence == 6 {
			return cut
		}
		return nil
	})
	if !errors.Is(err, cut) || !errors.Is(original.close(), cut) {
		t.Fatal("original operation did not retain its causal failure", err)
	}
	if raw, err := os.ReadFile(path); err != nil || len(raw) != 0 {
		t.Fatal("cut did not leave actual empty born file", err)
	}
	prefix, err := openSourceRecoveryPrefix(t.Context(), original.journal.path, "initial")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = prefix.close() })
	return prefix, path, body, cut
}

// Native presence routes to denial validation, never to successful Source
// admission. No account, generation, Release, State or manager is constructed.
func TestSourceRecoveryInitialEvidenceNativePresenceForcesRecoveredPath(t *testing.T) {
	original, _, _, _ := nativeSourceInitialPending(t, t.Context(), true)
	if err := original.close(); err != nil {
		t.Fatal(err)
	}
	journal := original.journal.path
	if present, err := sourceInitialRecoveryEvidencePresent(journal); present || err != nil {
		t.Fatal("ordinary original journal acquired synthetic recovery evidence", present, err)
	}
	control, result, guard := sourceCutPaths(journal)
	for _, path := range []string{control, result, guard, journal + ".recovery-completion"} {
		if err := os.WriteFile(path, []byte("incomplete evidence, not a Source proof\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if present, err := sourceInitialRecoveryEvidencePresent(journal); !present || err != nil {
			t.Fatal("one exact sidecar failed to require complete recovered validation", path, present, err)
		}
		if err := os.Rename(path, path+".retained"); err != nil {
			t.Fatal(err)
		}
	}
	// A real ENOTDIR is not ordinary absence; no permission/error seam is used.
	if present, err := sourceInitialRecoveryEvidencePresent(journal + "/not-a-directory"); present || err == nil {
		t.Fatal("unexpected native filesystem error treated as ordinary absence", present, err)
	}
}

func TestSourceRecoveryFileNativeContinuesOnlyOriginalEmptyBirth(t *testing.T) {
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	prefix, path, body, _ := nativeSourceRecoveryEmptyBirth(t)
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(prefix.journal.raw)
	writer, err := openSourceRecoveryBirths(prefix, strings.Repeat("a", 64), strings.Repeat("b", 64), map[string]bool{path: true})
	defer writer.close()
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.resumeSourceBornFile(body, prefix.check); err != nil {
		t.Fatal(err)
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || after.Mode().Perm() != 0444 {
		t.Fatal("continuation replaced original born inode or access", err)
	}
	if actual, err := os.ReadFile(path); err != nil || !bytes.Equal(actual, body) {
		t.Fatal("continuation changed authenticated intended bytes", err)
	}
	if actual, err := os.ReadFile(prefix.journal.path); err != nil || !bytes.Equal(actual, original) {
		t.Fatal("continuation rewrote original failed caller's prefix", err)
	}
	if len(writer.recovery.records) != 1 || writer.recovery.records[0].Phase != "file-ready" || writer.recovery.records[0].Inode != prefix.records[5].Inode {
		t.Fatal("ready evidence did not belong to independent continuation")
	}
	if err := writer.close(); err != nil {
		t.Fatal(err)
	}
}

func TestSourceRecoveryFileNativeRefusesUnknownOrSubstitutedWrite(t *testing.T) {
	for _, fault := range []string{"wrong-intended-bytes", "copied-inode", "partial-bytes", "full-unrecorded-bytes", "promoted-access", "canceled", "late-sync-failure"} {
		t.Run(fault, func(t *testing.T) {
			prefix, path, body, _ := nativeSourceRecoveryEmptyBirth(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			// The actual new prefix owner keeps its own context; cancellation
			// here is supplied as a genuine failed coordinator admission.
			if fault == "copied-inode" {
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "partial-bytes" || fault == "full-unrecorded-bytes" {
				visible := body
				if fault == "partial-bytes" {
					visible = body[:3]
				}
				if err := os.WriteFile(path, visible, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "promoted-access" {
				if err := os.Chmod(path, 0444); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writer, err := openSourceRecoveryBirths(prefix, strings.Repeat("a", 64), strings.Repeat("b", 64), map[string]bool{path: true})
			defer writer.close()
			if err != nil {
				t.Fatal(err)
			}
			if fault == "wrong-intended-bytes" {
				body = []byte("foreign intended content")
			}
			if fault == "canceled" {
				cancel()
			}
			late := errors.New("real sync completed but caller cannot prove completion")
			if fault == "late-sync-failure" {
				writer.afterSync = func(*os.File) error { return late }
			}
			err = writer.resumeSourceBornFile(body, ctx.Err)
			if err == nil || !errors.Is(writer.close(), err) {
				t.Fatal("uncertain continuation accepted or lost original failure", err)
			}
			actual, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if fault == "late-sync-failure" {
				if !errors.Is(err, late) || !bytes.Equal(actual, body) || len(writer.recovery.records) != 0 {
					t.Fatal("late failure erased visible bytes or claimed ready", err)
				}
			} else if !bytes.Equal(actual, before) {
				t.Fatal("refusal changed native original/substituted bytes")
			}
		})
	}
}
