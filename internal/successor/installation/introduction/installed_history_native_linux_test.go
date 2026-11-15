//go:build linux && introduction_installed_native

package introduction_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/introduction"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Uses the actual completed canonical start, never fabricated native provenance.
// Deliberate history damage belongs only to this disposable installed stand.
func checkIntroductionStartedHistory(t *testing.T, root, binding, history string, invoke func(...string) ([]byte, error)) {
	t.Helper()
	inspect := func() {
		t.Helper()
		raw, err := invoke("inspect-start-history", history, binding)
		if err != nil || !bytes.Equal(raw, []byte("{\"outcome\":\"introduction-start-history-verified\",\"runtime_authority\":false,\"retirement_proved\":false}\n")) {
			t.Fatal("canonical independent start-history observation", err, string(raw))
		}
	}
	inspect()
	v, err := release.OpenRetained(history)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	}()
	original, err := introduction.ReadStarted(t.Context(), v, binding, history)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close() // retain actual failure below, never claim a clean close
	if second, err := introduction.ReadStarted(t.Context(), v, binding, history); err == nil {
		_ = second.Close()
		t.Fatal("concurrent observer obtained original history leases")
	}
	journal := root + ".introduction-first-start-journal"
	retained, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	write := func(raw []byte) {
		t.Helper()
		f, err := os.OpenFile(journal, os.O_WRONLY|os.O_TRUNC, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, written := f.Write(raw)
		if err := errors.Join(written, f.Sync(), f.Close()); err != nil {
			t.Fatal(err)
		}
	}
	cut := bytes.LastIndexByte(retained[:len(retained)-1], '\n') + 1
	if cut <= 0 {
		t.Fatal("completed journal has no preceding record")
	}
	write(retained[:cut]) // retain archive, remove final ACK evidence
	if err := original.Check(); err == nil {
		t.Fatal("original observer accepted truncated start history")
	}
	write(retained)
	if err := original.Check(); err == nil {
		t.Fatal("restoration revived a failed original observer")
	}
	if err := original.Close(); err == nil {
		t.Fatal("original close lost its history failure")
	}
	// A separately admitted observer may read the restored original inode;
	// it never inherits the failed handle's custody or result.
	reopened, err := introduction.ReadStarted(t.Context(), v, binding, history)
	if err != nil {
		t.Fatal("independent restored history observation", err)
	}
	if err := errors.Join(reopened.Check(), reopened.Close()); err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(t.TempDir(), "original-journal")
	if err := os.Rename(journal, saved); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := os.Stat(saved); err == nil {
			_ = os.Remove(journal)
			if err := os.Rename(saved, journal); err != nil {
				t.Error(err)
			}
		}
	}()
	if absent, err := introduction.ReadStarted(t.Context(), v, binding, history); err == nil {
		_ = absent.Close()
		t.Fatal("missing original start journal accepted")
	}
	if err := os.WriteFile(journal, retained, 0600); err != nil {
		t.Fatal(err)
	}
	if foreign, err := introduction.ReadStarted(t.Context(), v, binding, history); err == nil {
		_ = foreign.Close()
		t.Fatal("same-byte foreign journal accepted as original")
	}
	if err := os.Remove(journal); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(saved, journal); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if stopped, err := introduction.ReadStarted(canceled, v, binding, history); err == nil {
		_ = stopped.Close()
		t.Fatal("canceled history reader accepted")
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	inspect()
}
