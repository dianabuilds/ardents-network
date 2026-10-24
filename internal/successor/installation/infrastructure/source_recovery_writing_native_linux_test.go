//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"syscall"
	"testing"
)

func TestSourceRecoveryWritingNativeBirthsAndPartialEvidence(t *testing.T) {
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	original, _, _, root := nativeSourceInitialPending(t, t.Context(), true)
	if err := original.close(); err != nil {
		t.Fatal(err)
	}
	prefix, err := openSourceRecoveryPrefix(t.Context(), original.journal.path, "initial")
	if err != nil {
		t.Fatal(err)
	}
	defer prefix.close()
	before := bytes.Clone(prefix.journal.raw)
	path := root + ".new-file"
	writer, err := openSourceRecoveryBirths(prefix, strings.Repeat("a", 64), strings.Repeat("b", 64), map[string]bool{path: true})
	defer writer.close()
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.write(path, []byte("actual new operation bytes"), 0444, 0, 0, prefix.check); err != nil {
		t.Fatal(err)
	}
	if len(writer.recovery.records) != 3 || writer.recovery.records[0].Sequence != 4 || writer.recovery.records[2].Phase != "file-ready" {
		t.Fatal("new birth did not emit its own continued sequence")
	}
	partial := bytes.Clone(writer.journal.body)
	if json.Valid(partial) {
		t.Fatal("partial operation published a complete envelope")
	}
	for _, path := range []string{prefix.journal.path, writer.journal.path} {
		probe, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		locked := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		closed := probe.Close()
		if !errors.Is(locked, syscall.EWOULDBLOCK) || closed != nil {
			t.Fatal("new/original writer exclusion unavailable", locked, closed)
		}
	}
	if err := writer.close(); err != nil {
		t.Fatal(err)
	}
	retained, err := os.ReadFile(writer.journal.path)
	if err != nil || !bytes.Equal(retained, partial) {
		t.Fatal("Close rewrote partial evidence", err)
	}
	if actual, err := os.ReadFile(prefix.journal.path); err != nil || !bytes.Equal(actual, before) {
		t.Fatal("fresh writer modified original prefix", err)
	}
	for i := 0; i < 8; i++ {
		baseline := sourceHistoryDescriptors(t, map[string]bool{writer.journal.path: true})
		retry, err := openSourceRecoveryBirths(prefix, strings.Repeat("a", 64), strings.Repeat("b", 64), nil)
		if !errors.Is(err, os.ErrExist) {
			t.Fatal("later caller adopted retained completion evidence", err)
		}
		_ = retry.close()
		if actual := sourceHistoryDescriptors(t, map[string]bool{writer.journal.path: true}); !reflect.DeepEqual(actual, baseline) {
			t.Fatal("refused acquisition retained descriptor without GC", baseline, actual)
		}
	}
}

// Only journal/envelope files are real here. Remaining numeric resource facts
// come from the independent byte vector and confer no native, retirement,
// Release, State or installed recovery proof. This isolates streamed framing.
func TestSourceRecoveryWritingNativeCompleteGrammarEnvelope(t *testing.T) {
	dummy, path := nativeSourceBirths(t, t.Context())
	if err := dummy.close(); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(filepath.Dir(path), "installation")
	full, configuration, oldHashes := sourceReplacementRecordVector(t)
	oldBinding := digest(sourceReplacementBindingVector())
	oldUnit := configuration.Configuration.UnitSHA256
	configuration.Configuration.Request.InstallationRoot = root
	request, err := json.Marshal(configuration.Configuration.Request)
	if err != nil {
		t.Fatal(err)
	}
	configuration.Configuration.RequestSHA256 = digest(append(request, '\n'))
	unit, err := RenderSourceUnit([]byte(SourceTemplate), configuration.Configuration.Request, configuration.Configuration.Generation)
	if err != nil {
		t.Fatal(err)
	}
	configuration.Configuration.UnitSHA256 = digest(unit)
	binding, err := json.Marshal(configuration)
	if err != nil {
		t.Fatal(err)
	}
	binding = append(binding, '\n')
	if _, err := decodeSourceReplacementBinding(binding); err != nil {
		t.Fatal("relocated byte oracle configuration", err)
	}
	hash := digest(binding)
	replacePath := func(path string) string { return strings.ReplaceAll(path, "/var/lib/install", root) }
	hashes := map[string]string{}
	for path, value := range oldHashes {
		if value == oldBinding {
			value = hash
		}
		if value == oldUnit {
			value = configuration.Configuration.UnitSHA256
		}
		hashes[replacePath(path)] = value
	}
	records, err := decodeSourceRecoveryPrefix(full, 55)
	if err != nil {
		t.Fatal(err)
	}
	journal := sourceReplacementJournal(root, configuration.Configuration.Generation)
	born, err := openSourceBirths(t.Context(), journal, nil)
	if err != nil {
		t.Fatal(err)
	}
	native := born.journal.info.Sys().(*syscall.Stat_t)
	if err := born.close(); err != nil {
		t.Fatal(err)
	}
	var transcript []byte
	for i := range records {
		records[i].Path = replacePath(records[i].Path)
		if records[i].SHA256 == oldBinding {
			records[i].SHA256 = hash
		}
		if records[i].SHA256 == oldUnit {
			records[i].SHA256 = configuration.Configuration.UnitSHA256
		}
		if records[i].Path == filepath.Join(root, "source-replacement-pending") {
			records[i].SHA256 = digest([]byte("ardents-source-replacement-pending-v1\n" + hash + "\n"))
		}
		if i == 0 {
			records[i].Device, records[i].Inode = uint64(native.Dev), native.Ino
		}
		line, err := json.Marshal(records[i])
		if err != nil {
			t.Fatal(err)
		}
		transcript = append(transcript, line...)
		transcript = append(transcript, '\n')
	}
	lines := bytes.SplitAfter(transcript, []byte{'\n'})
	original := bytes.Join(lines[:54], nil)
	if err := os.WriteFile(journal, original, 0600); err != nil {
		t.Fatal(err)
	}
	prefix, err := openSourceRecoveryPrefix(t.Context(), journal, "replacement")
	if err != nil {
		t.Fatal(err)
	}
	defer prefix.close()
	writer, err := openSourceRecoveryBirths(prefix, configuration.Configuration.Generation, hash, nil)
	defer writer.close()
	if err != nil {
		t.Fatal(err)
	}
	tail := records[54]
	if err := writer.record(tail.Phase, tail.Path, nil, tail.SHA256, os.FileMode(tail.Mode), tail.UID, tail.GID); err != nil {
		t.Fatal(err)
	}
	if err := writer.finishSourceRecoveryRecords(binding, hashes, strings.Repeat("3", 64), strings.Repeat("4", 64)); err != nil {
		t.Fatal("complete streamed grammar", err)
	}
	if err := writer.close(); err != nil {
		t.Fatal(err)
	}
	actual, err := prefix.readCompletion(configuration.Configuration.Generation, hash)
	if err != nil || !bytes.Equal(actual, transcript) {
		t.Fatal("independent reader changed original/suffix framing", err)
	}
	if actual, err := os.ReadFile(journal); err != nil || !bytes.Equal(actual, original) {
		t.Fatal("completed envelope changed original caller's prefix", err)
	}
}

func TestSourceRecoveryWritingNativeCloseWaitsAndRetainsLateFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	original, _, _, _ := nativeSourceInitialPending(t, t.Context(), true)
	if err := original.close(); err != nil {
		t.Fatal(err)
	}
	prefix, err := openSourceRecoveryPrefix(ctx, original.journal.path, "initial")
	if err != nil {
		t.Fatal(err)
	}
	writer, err := openSourceRecoveryBirths(prefix, strings.Repeat("a", 64), strings.Repeat("b", 64), nil)
	if err != nil {
		t.Fatal(errors.Join(err, writer.close(), prefix.close()))
	}
	closed := make(chan error, 1)
	go func() { closed <- prefix.close() }()
	waitSourceRecoverySealed(t, prefix)
	select {
	case err := <-closed:
		t.Fatal("prefix returned original lease before new writer physically closed", err)
	default:
	}
	cancel()
	if err := writer.journal.file.Close(); err != nil {
		t.Fatal(err)
	}
	err = writer.close()
	if !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("writer lost cancellation or actual Close failure", err)
	}
	err = <-closed
	if !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("prefix released borrowed lifetime without late writer failure", err)
	}
}
