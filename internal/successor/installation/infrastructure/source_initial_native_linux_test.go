//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// The checked runner invokes these only after the real compiled initial
// installer has completed on this private stand. The marker selects data;
// it provides no provenance, authorization, initial birth or startup right.
func nativeSourceInitial(t *testing.T, ctx context.Context) (*sourceInitial, *release.Verifier, string, string) {
	t.Helper()
	parent := os.Getenv("ARDENTS_SOURCE_STOPPED_FUTURE_PARENT")
	if os.Geteuid() != 0 || !sourcePath(parent) {
		t.Fatal("invalid environment: original private stopped fixture required")
	}
	raw, err := os.ReadFile(filepath.Join(parent, "source-initial-readback.json"))
	var fixture struct {
		Binding string `json:"binding"`
		History string `json:"history"`
	}
	if err != nil || len(raw) > 16<<10 || json.Unmarshal(raw, &fixture) != nil ||
		!sourcePath(fixture.Binding) || !sourcePath(fixture.History) ||
		!contains(parent, fixture.Binding) || !contains(parent, fixture.History) {
		t.Fatal("invalid environment: genuine completed fixture unavailable", err)
	}
	v, err := release.OpenRetained(fixture.History)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	})
	s, err := openSourceInitial(ctx, v, fixture.Binding, fixture.History)
	if err != nil {
		t.Fatal("genuine complete initial transcript readback", err)
	}
	t.Cleanup(func() { _ = s.close() })
	return s, v, fixture.Binding, fixture.History
}

func TestSourceInitialNativeCompleteTranscriptAndCausalRefusals(t *testing.T) {
	s, v, binding, history := nativeSourceInitial(t, t.Context())
	journal := s.selection.binding.Request.InstallationRoot + ".source-initial-journal"
	raw := append([]byte(nil), s.selection.files[journal].raw...)
	admitted, err := s.history.commitment(v, s.selection.files[binding].raw)
	if err != nil {
		t.Fatal(err)
	}
	hashes := map[string]string{}
	for path, file := range s.selection.files {
		hashes[path] = digest(file.raw)
	}
	b := s.selection.binding
	bindingSHA := digest(s.selection.files[binding].raw)
	owned, err := decodeSourceInitialRecords(raw, b, hashes, admitted)
	if err != nil || len(owned) != 24 {
		t.Fatal("complete original owned-resource inventory", len(owned), err)
	}
	// Every phase in the genuinely produced transcript must be consumed in
	// place. Merely finding its final stopped marker cannot pass this oracle.
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 112 {
		t.Fatal("independent finite transcript count", len(lines))
	}
	for i, line := range lines {
		var record sourceRecord
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		record.Phase += "-foreign"
		changed, _ := json.Marshal(record)
		mutated := append([][]byte(nil), lines...)
		mutated[i] = changed
		if _, err := decodeSourceInitialRecords(append(bytes.Join(mutated, []byte{'\n'}), '\n'), b, hashes, admitted); err == nil {
			t.Fatal("foreign phase accepted at original sequence", i)
		}
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	for name, changed := range map[string][]byte{
		"torn": raw[:len(raw)-1], "missing-initial": append(bytes.Join(lines[1:], []byte{'\n'}), '\n'),
		"extra-tail":                  append(append(append([]byte(nil), raw...), lines[len(lines)-1]...), '\n'),
		"duplicate-field":             bytes.Replace(raw, []byte(`"sequence":0,`), []byte(`"sequence":0,"sequence":0,`), 1),
		"foreign-field":               bytes.Replace(raw, []byte(`"sequence":0,`), []byte(`"foreign":0,"sequence":0,`), 1),
		"old-binding-only-commitment": bytes.Replace(raw, []byte(admitted), []byte(bindingSHA), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(journal, changed, 0600); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.WriteFile(journal, raw, 0600); err != nil {
					t.Error(err)
				}
			}()
			got, err := openSourceInitial(t.Context(), v, binding, history)
			if got != nil || !errors.Is(err, ErrBinding) {
				_ = got.close()
				t.Fatal("malformed/old original transcript accepted", err)
			}
		})
	}
	t.Run("original-verifier", func(t *testing.T) {
		original, err := openSourceInitial(t.Context(), v, binding, history)
		if err != nil {
			t.Fatal(err)
		}
		defer original.close()
		if err := v.Close(); err != nil {
			t.Fatal(err)
		}
		fresh, err := release.OpenRetained(history)
		if err != nil {
			t.Fatal(err)
		}
		defer fresh.Close()
		if err := original.check(); !errors.Is(err, release.ErrClosed) {
			t.Fatal("fresh verifier of same history revived original lifetime", err)
		}
		if err := original.close(); !errors.Is(err, release.ErrClosed) {
			t.Fatal("original verifier loss erased during physical close", err)
		}
	})
}

func TestSourceInitialNativeHeldReadPreservesCancellationAndClose(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s, _, _, _ := nativeSourceInitial(t, ctx)
	journal := s.selection.binding.Request.InstallationRoot + ".source-initial-journal"
	entered, resume := make(chan struct{}), make(chan struct{})
	read := 0
	s.selection.observed = func(_ *os.File) {
		read++
		if read == len(s.selection.files) {
			close(entered)
			<-resume
		}
	}
	checked := make(chan error, 1)
	go func() { checked <- s.check() }()
	<-entered
	cancel()
	if err := s.selection.files[journal].file.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-checked:
		t.Fatal("held original snapshot read returned before join")
	default:
	}
	close(resume)
	if err := <-checked; !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("original cancellation or actual native failure lost", err)
	}
	first := s.close()
	if !errors.Is(first, context.Canceled) || !errors.Is(first, os.ErrClosed) || s.close() != first {
		t.Fatal("independent close failure or original retained result lost", first)
	}
}
