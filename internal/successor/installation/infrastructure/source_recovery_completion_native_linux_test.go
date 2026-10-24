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
	"sync"
	"syscall"
	"testing"
)

// The envelope below deliberately has no valid operation grammar. Actual
// ext4/descriptor/flock observations test file custody, not a successful Source
// installation, Release decision, completed recovery or domain join.
func nativeSourceRecoveryEnvelope(t *testing.T, owner *sourceRecoveryPrefix) (string, string, []byte) {
	t.Helper()
	generation, binding := strings.Repeat("a", 64), strings.Repeat("b", 64)
	native := owner.journal.info.Sys().(*syscall.Stat_t)
	records := make(sourceRecoveryRecords, 111)
	transcript := bytes.Clone(owner.journal.raw)
	for i := range records {
		records[i] = sourceRecord{Sequence: i + 1, Phase: "codec-only", Path: "/not-an-installed-resource"}
		line, err := json.Marshal(records[i])
		if err != nil {
			t.Fatal(err)
		}
		transcript = append(transcript, line...)
		transcript = append(transcript, '\n')
	}
	completion := sourceRecoveryCompletion{Schema: "ardents-source-recovery-completion-v1", Operation: "initial",
		Journal: owner.journal.path, Device: uint64(native.Dev), Inode: native.Ino,
		PrefixSHA256: digest(owner.journal.raw), PrefixRecords: 1, Generation: generation, BindingSHA256: binding,
		Records: records, TranscriptSHA256: digest(transcript)}
	file, err := os.OpenFile(owner.journal.path+".recovery-completion", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	raw := marshalSourceRecoveryCodec(t, completion)
	n, written := file.Write(raw)
	if err := errors.Join(written, file.Sync(), file.Close()); err != nil || n != len(raw) {
		t.Fatal("native envelope fixture write", err)
	}
	return generation, binding, transcript
}

func nativeSourceRecoveryPrefix(t *testing.T, ctx context.Context) *sourceRecoveryPrefix {
	t.Helper()
	births, _ := nativeSourceBirths(t, t.Context())
	if err := births.close(); err != nil {
		t.Fatal(err)
	}
	owner, err := openSourceRecoveryPrefix(ctx, births.journal.path, "initial")
	if err != nil {
		t.Fatal(err)
	}
	return owner
}

func TestSourceRecoveryCompletionNativeCustodyAndCrossFileReobservation(t *testing.T) {
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	owner := nativeSourceRecoveryPrefix(t, t.Context())
	defer owner.close()
	generation, binding, expected := nativeSourceRecoveryEnvelope(t, owner)
	actual, err := owner.readCompletion(generation, binding)
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("native completion changed detached canonical bytes", err)
	}
	// Syntactic completion is deliberately insufficient for the unchanged full
	// Source grammar. No native fixture here grants semantic authority.
	if _, err := decodeSourceInitialRecords(actual, sourceBinding{}, map[string]string{}, strings.Repeat("c", 64)); err == nil {
		t.Fatal("file envelope alone entered the complete initial grammar")
	}
	probes := map[string]*os.File{}
	for _, path := range []string{owner.journal.path, owner.completion.path} {
		probe, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		probes[path] = probe
		defer probe.Close()
		if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Fatal("original journal/completion writer exclusion missing", err)
		}
	}
	var changed sync.Once
	owner.observed = func(file *os.File) {
		if file.Name() == owner.completion.path {
			changed.Do(func() {
				if err := os.Chmod(owner.journal.path, 0644); err != nil {
					t.Error(err)
				}
			})
		}
	}
	if err := owner.check(); !errors.Is(err, ErrBinding) {
		t.Fatal("later completion read concealed a changed earlier journal", err)
	}
	if err := os.Chmod(owner.journal.path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := owner.check(); !errors.Is(err, ErrBinding) {
		t.Fatal("restoring journal appearance erased original refusal", err)
	}
	if err := owner.close(); !errors.Is(err, ErrBinding) {
		t.Fatal("Close lost cross-file native refusal", err)
	}
	for _, probe := range probes {
		if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			t.Fatal("joined Close retained journal/completion flock before GC", err)
		}
		if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_UN); err != nil {
			t.Fatal(err)
		}
	}
	watched := map[string]bool{owner.journal.path: true, owner.completion.path: true, filepath.Dir(owner.journal.path): true}
	expectedDescriptors := map[string]int{owner.journal.path: 1, owner.completion.path: 1}
	if actual := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(actual, expectedDescriptors) {
		t.Fatal("completion owner leaked descriptors before GC", expectedDescriptors, actual)
	}
}

func TestSourceRecoveryCompletionNativeAcquisitionRefusals(t *testing.T) {
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	for _, fault := range []string{"missing", "live-writer", "access", "symlink"} {
		t.Run(fault, func(t *testing.T) {
			owner := nativeSourceRecoveryPrefix(t, t.Context())
			defer owner.close()
			generation, binding := strings.Repeat("a", 64), strings.Repeat("b", 64)
			path := owner.journal.path + ".recovery-completion"
			if fault != "missing" {
				generation, binding, _ = nativeSourceRecoveryEnvelope(t, owner)
			}
			var writer *os.File
			switch fault {
			case "live-writer":
				var err error
				writer, err = os.OpenFile(path, os.O_RDWR, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer writer.Close()
				if err := syscall.Flock(int(writer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
					t.Fatal(err)
				}
			case "access":
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Rename(path, path+".retained"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".retained", path); err != nil {
					t.Fatal(err)
				}
			}
			actual, err := owner.readCompletion(generation, binding)
			if err == nil || actual != nil || fault == "live-writer" && !errors.Is(err, syscall.EWOULDBLOCK) {
				t.Fatal("foreign/incomplete completion acquisition accepted", err)
			}
			if err := owner.close(); err == nil {
				t.Fatal("completion refusal disappeared at Close")
			}
			watched := map[string]bool{owner.journal.path: true, path: true, path + ".retained": true, filepath.Dir(path): true}
			counts := map[string]int{}
			if writer != nil {
				counts[path] = 1 // The independently owned writer must survive.
			}
			if actual := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(actual, counts) {
				t.Fatal("refused acquisition leaked or closed a foreign descriptor", counts, actual)
			}
		})
	}
}

func TestSourceRecoveryCompletionNativeCanceledReadJoinsBeforeReturn(t *testing.T) {
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	owner := nativeSourceRecoveryPrefix(t, ctx)
	defer owner.close()
	generation, binding, _ := nativeSourceRecoveryEnvelope(t, owner)
	probe, err := os.Open(owner.journal.path)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	owner.observed = func(file *os.File) {
		if file.Name() == owner.journal.path+".recovery-completion" {
			close(entered)
			<-release
		}
	}
	type readResult struct {
		raw []byte
		err error
	}
	read := make(chan readResult, 1)
	go func() {
		raw, err := owner.readCompletion(generation, binding)
		read <- readResult{raw, err}
	}()
	<-entered
	cancel()
	if err := owner.check(); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation admitted another original observation", err)
	}
	if err := owner.completion.file.Close(); err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- owner.close() }()
	waitSourceRecoverySealed(t, owner)
	select {
	case err := <-closed:
		t.Fatal("Close returned before original completion read joined", err)
	default:
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("original journal exclusion returned before completion read join", err)
	}
	unblock()
	result := <-read
	if result.raw != nil || !errors.Is(result.err, context.Canceled) || !errors.Is(result.err, os.ErrClosed) {
		t.Fatal("late completion produced bytes or lost original failure", result.err)
	}
	if err := <-closed; !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("completion Close lost cancellation/native read/cleanup failures", err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal("joined Close retained original exclusion", err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	watched := map[string]bool{owner.journal.path: true, owner.journal.path + ".recovery-completion": true, filepath.Dir(owner.journal.path): true}
	if actual := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(actual, map[string]int{owner.journal.path: 1}) {
		t.Fatal("canceled completion retained descriptors before GC", actual)
	}
}
