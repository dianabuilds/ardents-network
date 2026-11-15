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

// Canonical Stop must obtain the original domain report and native join itself;
// systemctl cleanup is not the oracle. Both semantic histories remain byte-exact.
func checkIntroductionExplicitStop(t *testing.T, root, binding, history, spend, slots string, invoke func(...string) ([]byte, error)) {
	t.Helper()
	if raw, err := invoke("inspect-retirement", history, binding); err == nil {
		t.Fatal("live invocation acquired retirement", string(raw))
	}
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	spendPath, slotsPath := filepath.Join(spend, "closed-token-spends"), filepath.Join(slots, "closed-introduction-slots")
	beforeSpend, beforeSlots := read(spendPath), read(slotsPath)
	if raw, err := invoke("stop", history, binding); err != nil || !bytes.Equal(raw, []byte("{\"outcome\":\"introduction-original-stop-completed\",\"restart_admitted\":false}\n")) {
		t.Fatal("canonical original domain/native Stop", err, string(raw))
	}
	if !bytes.Equal(beforeSpend, read(spendPath)) || !bytes.Equal(beforeSlots, read(slotsPath)) {
		t.Fatal("Stop changed original spend or slot floors")
	}
	for range 2 {
		if raw, err := invoke("inspect-retirement", history, binding); err != nil || !bytes.Equal(raw, []byte("{\"outcome\":\"introduction-retirement-verified\",\"restart_admitted\":false}\n")) {
			t.Fatal("independent complete retirement", err, string(raw))
		}
	}
	journal := root + ".introduction-retirement-00000000-journal"
	original := read(journal)
	if raw, err := invoke("stop", history, binding); err == nil {
		t.Fatal("second Stop adopted retired invocation", string(raw))
	}
	if !bytes.Equal(original, read(journal)) {
		t.Fatal("repeated Stop rewrote original retirement")
	}
	directory := filepath.Join(root, "introduction-retirement-00000000")
	pending := filepath.Join(directory, "retirement-pending")
	if err := os.WriteFile(pending, []byte("uncertain original completion\n"), 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(pending)
	if raw, err := invoke("inspect-retirement", history, binding); err == nil {
		t.Fatal("retained blocker accepted as committed retirement", string(raw))
	}
	if err := os.Remove(pending); err != nil {
		t.Fatal(err)
	}
	if raw, err := invoke("inspect-retirement", history, binding); err != nil {
		t.Fatal("separate original retirement reader", err, string(raw))
	}
	checkIntroductionRetirementCustody(t, directory, binding, history)
}

func checkIntroductionRetirementCustody(t *testing.T, directory, binding, history string) {
	t.Helper()
	v, err := release.OpenRetained(history)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	}()
	open := func(ctx context.Context) introduction.Started {
		t.Helper()
		h, err := introduction.ReadStarted(ctx, v, binding, history)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	path := filepath.Join(directory, "retirement-result.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write := func(raw []byte) {
		t.Helper()
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
		if err != nil {
			t.Fatal(err)
		}
		n, written := f.Write(raw)
		if err := errors.Join(written, f.Sync(), f.Close()); err != nil || n != len(raw) {
			t.Fatal("retirement test mutation", err)
		}
	}
	h := open(t.Context())
	defer h.Close()
	write(original[:len(original)-1])
	if h.CheckRetirement() == nil {
		t.Fatal("changed original retirement result accepted")
	}
	write(original)
	checkErr, closeErr := h.CheckRetirement(), h.Close()
	if checkErr == nil || closeErr == nil {
		t.Fatal("restoration erased original retirement reader failure")
	}
	ctx, cancel := context.WithCancel(t.Context())
	h = open(ctx)
	cancel()
	checkErr, closeErr = h.CheckRetirement(), h.Close()
	if checkErr == nil || closeErr == nil {
		t.Fatal("canceled retirement reader retained success")
	}
	saved := filepath.Join(t.TempDir(), "original-retirement-result")
	if err := os.Rename(path, saved); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := os.Stat(saved); err == nil {
			_ = os.Remove(path)
			if err := os.Rename(saved, path); err != nil {
				t.Error(err)
			}
		}
	}()
	if absent, err := introduction.ReadStarted(t.Context(), v, binding, history); err == nil {
		_ = absent.Close()
		t.Fatal("missing original retirement result accepted")
	}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if foreign, err := introduction.ReadStarted(t.Context(), v, binding, history); err == nil {
		_ = foreign.Close()
		t.Fatal("same-byte foreign retirement inode accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(saved, path); err != nil {
		t.Fatal(err)
	}
	h = open(t.Context())
	if err := errors.Join(h.CheckRetirement(), h.Close()); err != nil {
		t.Fatal("fresh independent original retirement reader", err)
	}
}
