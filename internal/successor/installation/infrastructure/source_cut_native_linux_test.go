//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// This genuine original-controller result comes from the actual test process
// and a native file prefix (two records by default, at most55 for the outside
// observation control). Repeated framing observations are not a valid Source
// initial transcript. This cannot satisfy Source's account,
// inventory, Release, State or manager admission and is no installed recovery.
func sourceCutNativeProof(t *testing.T, ctx context.Context, cancel context.CancelFunc, program *os.File, channel *net.UnixConn, scope *sourceTransitionScope, nonce string, deadline time.Time, scenario string) {
	t.Helper()
	closed := false
	defer func() {
		if !closed {
			if err := errors.Join(program.Close(), channel.Close()); err != nil {
				t.Error(err)
			}
		}
	}()
	origin, err := captureSourceTransitionOrigin(program, scope)
	if err != nil {
		t.Fatal(err)
	}
	completedProof := scenario == "cut-proof" || strings.HasPrefix(scenario, "cut-proof-prefix-") || scenario == "external-cut"
	if scenario != "external-cut" {
		scope.frozen = cancel
	}
	cut, err := controlSourceTransition(ctx, scope, channel, nonce, "initial", deadline)
	if cut != nil {
		defer cut.close()
	}
	if !errors.Is(err, context.Canceled) || cut == nil || !scope.joined || scope.reaped != 1 {
		t.Fatal("genuine frozen cancellation/join missing", cut, scope.reaped, err)
	}
	before := bytes.Clone(cut.prefix.journal.raw)
	if err := errors.Join(program.Close(), channel.Close()); err != nil {
		closed = true
		t.Fatal(err)
	}
	closed = true
	controlPath, resultPath, pendingPath := sourceCutPaths(cut.message.Journal)
	uncertain := errors.New("original proof Sync result uncertain after actual fsync")
	if !completedProof {
		cut.afterSync = func(file *os.File) error {
			if file.Name() != controlPath {
				return nil
			}
			raw, err := os.ReadFile(controlPath)
			if err != nil {
				return err
			}
			if !bytes.Contains(raw, []byte(`"phase":"original-cut-completed"`)) {
				return nil
			}
			if scenario == "cut-proof-sync-uncertain" {
				return uncertain
			}
			return file.Close() // actual later writer readback/Close must fail
		}
	}
	completion, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	err = publishSourceCutResult(completion, cut, scope, origin)
	if present, evidenceErr := sourceInitialRecoveryEvidencePresent(cut.message.Journal); !present || evidenceErr != nil {
		t.Fatal("genuine guarded/complete cut without suffix escaped recovered validation", present, evidenceErr)
	}
	if completedProof && err != nil {
		t.Fatal("genuine original cut publication", err)
	}
	if !completedProof && err == nil {
		t.Fatal("uncertain original proof accepted")
	}
	if scenario == "cut-proof-sync-uncertain" && !errors.Is(err, uncertain) {
		t.Fatal("first proof Sync error lost", err)
	}
	_, pendingErr := os.Lstat(pendingPath)
	if completedProof && !errors.Is(pendingErr, os.ErrNotExist) {
		t.Fatal("completed proof still blocked", pendingErr)
	}
	if !completedProof && pendingErr != nil {
		t.Fatal("uncertain proof lost its original blocker", pendingErr)
	}
	after, err := os.ReadFile(cut.message.Journal)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("original interrupted journal changed", err)
	}
	controlRaw, err := os.ReadFile(controlPath)
	if err != nil {
		t.Fatal(err)
	}
	resultRaw, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	controlInfo, err := os.Lstat(controlPath)
	if err != nil {
		t.Fatal(err)
	}
	resultInfo, err := os.Lstat(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(cut.message.Journal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeSourceCutResult(controlRaw, resultRaw, after, controlInfo, resultInfo, info, cut.message.Journal, "initial", cut.message.Generation, cut.message.BindingSHA256, origin.ProgramSHA256); err != nil {
		t.Fatal("complete ordered physical result codec", err)
	}
	if strings.HasPrefix(scenario, "cut-proof-prefix-") {
		sourceCutNativeControlPrefix(t, completion, cut.message, origin.ProgramSHA256, scenario)
		return
	}
	// Complete-looking result bytes also exist in the two fault cases. The
	// retained guard is mandatory; the codec alone supplies no permission.
	for range 8 {
		reader, e := openSourceRecoveryPrefix(completion, cut.message.Journal, "initial")
		if e != nil {
			t.Fatal("fresh independent original prefix", e)
		}
		e = reader.readOriginalCut(cut.message.Generation, cut.message.BindingSHA256, origin.ProgramSHA256)
		if completedProof && e != nil {
			t.Error("independent physical proof reader", e)
		}
		if !completedProof && e == nil {
			t.Error("guarded complete-looking proof accepted")
		}
		if e == nil {
			if sourceInitialRecoveryPrefixSupported(reader.records) {
				t.Error("genuine early native cut widened supported Source continuation")
			}
			// These real file/process facts include no genuine Source manager.
			if reader.checkCutManager("") == nil {
				t.Error("native file proof supplied missing manager admission")
			}
			probe, pe := os.Open(controlPath)
			if pe != nil {
				t.Error(pe)
			} else {
				locked := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
				if locked == nil {
					t.Error("proof reader did not retain independent shared exclusion")
				}
				if ce := probe.Close(); ce != nil {
					t.Error(ce)
				}
			}
		}
		if ce := reader.close(); e == nil && ce != nil {
			t.Error("successful proof custody close", ce)
		}
		for _, target := range []string{cut.message.Journal, controlPath, resultPath, pendingPath} {
			if actual := sourceHistoryDescriptors(t, map[string]bool{target: true}); len(actual) != 0 {
				t.Fatal("repeated proof reader retained descriptors before GC", target, actual)
			}
		}
	}
	for _, target := range []string{cut.message.Journal, controlPath, resultPath, pendingPath} {
		if actual := sourceHistoryDescriptors(t, map[string]bool{target: true}); len(actual) != 0 {
			t.Fatal("proof publication retained descriptors before GC", target, actual)
		}
	}
	copyPath := resultPath + ".foreign-copy"
	if err := os.WriteFile(copyPath, resultRaw, 0600); err != nil {
		t.Fatal(err)
	}
	copyInfo, err := os.Lstat(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeSourceCutResult(controlRaw, resultRaw, after, controlInfo, copyInfo, info, cut.message.Journal, "initial", cut.message.Generation, cut.message.BindingSHA256, origin.ProgramSHA256); err == nil {
		t.Fatal("copied result lineage accepted")
	}
	if scenario == "cut-proof" {
		sourceCutNativeReadCancellation(t, cut.message, origin.ProgramSHA256)
		sourceCutNativeCopiedResult(t, completion, cut.message, origin.ProgramSHA256, copyPath)
	}
	t.Log("retained genuine native original cut", scenario, controlPath, "blocker", pendingErr == nil)
}

func sourceCutNativeReadCancellation(t *testing.T, boundary sourceTransitionMessage, program string) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	reader, err := openSourceRecoveryPrefix(ctx, boundary.Journal, "initial")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.close()
	control, result, _ := sourceCutPaths(boundary.Journal)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	reader.observed = func(file *os.File) {
		if file.Name() == control {
			close(entered)
			<-release
		}
	}
	checked := make(chan error, 1)
	go func() { checked <- reader.readOriginalCut(boundary.Generation, boundary.BindingSHA256, program) }()
	select {
	case <-entered:
	case err := <-checked:
		t.Fatal("physical proof read did not enter original control observation", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	cancel()
	if err := reader.cutResult.file.Close(); err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- reader.close() }()
	waitSourceRecoverySealed(t, reader)
	select {
	case err := <-closed:
		t.Fatal("proof Close returned before physical read join", err)
	default:
	}
	unblock()
	if err := <-checked; !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("proof read lost original cancellation or native failure", err)
	}
	if err := <-closed; !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("proof Close lost original cancellation or cleanup failure", err)
	}
	for _, path := range []string{boundary.Journal, control, result} {
		if actual := sourceHistoryDescriptors(t, map[string]bool{path: true}); len(actual) != 0 {
			t.Fatal("canceled proof read retained descriptors before GC", path, actual)
		}
		probe, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		locked := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err := errors.Join(locked, probe.Close()); err != nil {
			t.Fatal("canceled proof reader retained native exclusion", path, err)
		}
	}
}

func sourceCutNativeCopiedResult(t *testing.T, ctx context.Context, boundary sourceTransitionMessage, program, copyPath string) {
	t.Helper()
	_, result, _ := sourceCutPaths(boundary.Journal)
	saved := result + ".original-retained"
	if err := os.Rename(result, saved); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Rename(saved, result); err != nil {
			t.Error("retain original result name", err)
		}
	}()
	if err := os.Rename(copyPath, result); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Rename(result, copyPath); err != nil {
			t.Error("retain copied result evidence", err)
		}
	}()
	for range 8 {
		reader, err := openSourceRecoveryPrefix(ctx, boundary.Journal, "initial")
		if err != nil {
			t.Fatal(err)
		}
		err = reader.readOriginalCut(boundary.Generation, boundary.BindingSHA256, program)
		closed := reader.close()
		if !errors.Is(err, ErrBinding) || !errors.Is(closed, ErrBinding) {
			t.Fatal("copied same-byte result native lineage accepted", err, closed)
		}
		control, _, _ := sourceCutPaths(boundary.Journal)
		for _, path := range []string{boundary.Journal, control, result} {
			if actual := sourceHistoryDescriptors(t, map[string]bool{path: true}); len(actual) != 0 {
				t.Fatal("copied-result refusal retained descriptors before GC", path, actual)
			}
		}
	}
}
