//go:build installation_native

package endpoint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestInstallationNativeStartIntentionRetainsOriginalBarrierBeforeEffect(t *testing.T) {
	t.Parallel()
	stage := startBarrierFixture(t)
	barrier, err := prepareNativeStartBarrier(t.Context(), stage)
	stage.barrier = barrier
	if err != nil {
		t.Fatal(err)
	}
	if err := barrier.recordStartAttempt(); err != nil {
		t.Fatal(err)
	}
	// Independent byte oracle; a journal receipt does not manufacture a started
	// invocation, manager success, Release proof or an ACK.
	expected := []byte(fmt.Sprintf("{\"schema\":\"ardents-endpoint-installation-start-attempt-v1\",\"intent_digest\":\"%x\",\"socket_record_digest\":\"%x\"}\n", sha256.Sum256(stage.intent.body), sha256.Sum256(barrier.record.body)))
	body := stage.journal.Bytes(journal.Transitions, "start-attempt.json")
	if !bytes.Equal(body, expected) || !barrier.startRecorded || barrier.ackAttempted || barrier.ackWritten || stage.archivedIntent {
		t.Fatal("intention bytes differ or minted a completed start")
	}
	filename := filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "start-attempt.json")
	before, err := os.Lstat(filename)
	if err != nil || before.Mode() != 0600 || before.Sys().(*syscall.Stat_t).Uid != 0 || before.Sys().(*syscall.Stat_t).Gid != 0 || before.Sys().(*syscall.Stat_t).Nlink != 1 {
		t.Fatal("intention lost private original identity", err)
	}
	if err := stage.journal.Resync(t.Context(), journal.Transitions, "start-attempt.json"); err != nil {
		t.Fatal(err)
	}
	after, err := os.Lstat(filename)
	if err != nil || !os.SameFile(before, after) || before.Sys().(*syscall.Stat_t).Ctim != after.Sys().(*syscall.Stat_t).Ctim {
		t.Fatal("resync replaced original intention", err)
	}
	if err := barrier.recordStartAttempt(); !errors.Is(err, ErrBinding) {
		t.Fatal("same barrier permitted another start intention", err)
	}
	if err := barrier.close(); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("physical close erased start uncertainty", err)
	}
}

func TestInstallationNativeStartIntentionRefusesChangedBarrierAfterJournalIO(t *testing.T) {
	t.Parallel()
	stage := startBarrierFixture(t)
	ctx := &completionCleanupContext{Context: t.Context()}
	barrier, err := prepareNativeStartBarrier(ctx, stage)
	stage.barrier = barrier
	if err != nil {
		t.Fatal(err)
	}
	mutated := false
	ctx.observe = func() {
		if !mutated && len(stage.journal.Bytes(journal.Transitions, "start-attempt.json")) != 0 {
			mutated = true
			filename := filepath.Join(stage.lease.path, "start-completion.socket")
			if err := os.Chmod(filename, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(filename, 0660); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := barrier.recordStartAttempt(); !errors.Is(err, ErrBinding) || !mutated || barrier.startRecorded {
		t.Fatal("journal I/O renewed changed socket custody", err)
	}
	if err := barrier.recordStartAttempt(); !errors.Is(err, ErrBinding) || barrier.startRecorded {
		t.Fatal("fresh call renewed refused start", err)
	}
}

func TestInstallationNativeStartIntentionRetainsCancellationBeforeAndAfterWrite(t *testing.T) {
	t.Parallel()
	for _, afterWrite := range []bool{false, true} {
		t.Run(fmt.Sprint(afterWrite), func(t *testing.T) {
			t.Parallel() // Independent original journal/socket; no manager effects.
			stage := startBarrierFixture(t)
			base, cancel := context.WithCancel(t.Context())
			defer cancel()
			ctx := &completionCleanupContext{Context: base}
			barrier, err := prepareNativeStartBarrier(ctx, stage)
			stage.barrier = barrier
			if err != nil {
				t.Fatal(err)
			}
			if afterWrite {
				ctx.observe = func() {
					if len(stage.journal.Bytes(journal.Transitions, "start-attempt.json")) != 0 {
						cancel()
					}
				}
			} else {
				cancel()
			}
			if err := barrier.recordStartAttempt(); !errors.Is(err, context.Canceled) || barrier.startRecorded || barrier.ackAttempted {
				t.Fatal("cancelled intention admitted an effect", err)
			}
			filename := filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "start-attempt.json")
			_, recordErr := os.Lstat(filename)
			if afterWrite && recordErr != nil || !afterWrite && !errors.Is(recordErr, os.ErrNotExist) {
				t.Fatal("cancelled write lost original uncertainty", recordErr)
			}
			if err := observeStagedFile(stage.lease.root, "start-guard.json", barrier.guard); err != nil {
				t.Fatal("cancelled intention erased guard", err)
			}
		})
	}
}

// Filesystem/Unix-socket mechanism only. Fixture phases are not successful
// manager, Release, predecessor, started invocation or completion evidence.
func startBarrierFixture(t *testing.T) *installationTransaction {
	t.Helper()
	stage, filename, previous, body := successorSelectionFixture(t, true)
	if err := stage.recordFixedReplacement(t.Context(), filename, previous, previous.file.body, body); err != nil {
		t.Fatal(err)
	}
	written, err := stage.replaceRecordedFixedFile(t.Context(), filename, previous, previous.file.body, body)
	if err != nil {
		t.Fatal(err)
	}
	stage.fixed[filename] = fixedFileObservation{parent: previous.parent, file: written}
	for _, phase := range []struct{ name, value string }{{"0006.json", "reloading-manager"}, {"0007.json", "successor-reloaded-stopped"}} {
		body, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: phase.value})
		if err != nil {
			t.Fatal(err)
		}
		if err := stage.journal.Write(t.Context(), journal.Transitions, phase.name, body); err != nil {
			t.Fatal(err)
		}
	}
	return stage
}

func TestInstallationNativeRecordedCompletionCleanupClosesBeforeRemoval(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	stage := startBarrierFixture(t)
	ctx := &completionCleanupContext{Context: t.Context()}
	barrier, err := prepareNativeStartBarrier(ctx, stage)
	stage.barrier = barrier
	if err != nil {
		t.Fatal(err)
	}
	client, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: filepath.Join(stage.lease.path, "start-completion.socket"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	barrier.connection, err = barrier.listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	originalListener, originalConnection := barrier.listener, barrier.connection
	originalFiles := map[string]os.FileInfo{
		"start-completion.socket": barrier.socket,
		"start-socket.json":       barrier.record.identity,
		"start-guard.json":        barrier.guard.identity,
	}
	observedBeforeUnlink, physicallyClosed := false, false
	ctx.observe = func() {
		if barrier.cleanupAttempted && !observedBeforeUnlink {
			observedBeforeUnlink = true
			listenerRaw, listenerErr := originalListener.SyscallConn()
			if listenerErr == nil {
				listenerErr = listenerRaw.Control(func(uintptr) {})
			}
			connectionRaw, connectionErr := originalConnection.SyscallConn()
			if connectionErr == nil {
				connectionErr = connectionRaw.Control(func(uintptr) {})
			}
			_, pathErr := stage.lease.root.Lstat("start-completion.socket")
			physicallyClosed = errors.Is(listenerErr, net.ErrClosed) && errors.Is(connectionErr, net.ErrClosed) && pathErr == nil
		}
	}
	// This is physical cleanup only: no accepted process, archived start or
	// full ACK is supplied. It must not become root transition authority.
	if err := barrier.cleanupArchivedStart(stage, &installedProcessPin{}); !errors.Is(err, ErrBinding) || barrier.cleanupAttempted {
		t.Fatal("unqualified facts admitted post-ACK cleanup", err)
	}
	if err := barrier.retireRecordedCompletion(); err != nil {
		t.Fatal(err)
	}
	if !observedBeforeUnlink || !physicallyClosed {
		t.Fatal("original socket pathname removal preceded physical close")
	}
	if !barrier.cleaned || !barrier.cleanupAttempted || barrier.listener != nil || barrier.connection != nil {
		t.Fatal("original physical completion handles remain")
	}
	var first [1]byte
	if n, err := client.Read(first[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatal("cleanup fabricated ACK or left connection alive", n, err)
	}
	for _, name := range []string{"start-completion.socket", "start-socket.json", "start-guard.json"} {
		if _, err := stage.lease.root.Lstat(name); !errors.Is(err, syscall.ENOENT) {
			t.Fatal("recorded completion path remains", name, err)
		}
	}
	for recordName, name := range map[string]string{
		"completion-socket-removal.json": "start-completion.socket",
		"completion-record-removal.json": "start-socket.json",
		"completion-guard-removal.json":  "start-guard.json",
	} {
		var recorded completionRemoval
		body := stage.journal.Bytes(journal.Transitions, recordName)
		if err := json.Unmarshal(body, &recorded); err != nil {
			t.Fatal("original removal provenance missing", recordName, err)
		}
		native := originalFiles[name].Sys().(*syscall.Stat_t)
		if recorded.Schema != "ardents-endpoint-installation-completion-removal-v1" ||
			recorded.Name != name || recorded.Device != uint64(native.Dev) || recorded.Inode != native.Ino ||
			recorded.UID != native.Uid || recorded.GID != native.Gid || recorded.Mode != native.Mode ||
			recorded.Size != native.Size || recorded.ChangedSec != native.Ctim.Sec || recorded.ChangedNSec != native.Ctim.Nsec ||
			recorded.ModifiedSec != native.Mtim.Sec || recorded.ModifiedNSec != native.Mtim.Nsec ||
			recorded.IntentDigest != digestHex(barrier.guard.body) {
			t.Fatal("removal provenance adopted different custody", recorded)
		}
	}
	if err := barrier.observe(); !errors.Is(err, ErrBinding) || barrier.ackAttempted || barrier.ackWritten || stage.archivedIntent {
		t.Fatal("physical cleanup minted an accepted transition", err)
	}
	if err := barrier.retireRecordedCompletion(); !errors.Is(err, ErrBinding) {
		t.Fatal("physical cleanup allowed another attempt", err)
	}
	if _, err := stage.lease.writer.Stat(); err != nil {
		t.Fatal("completion cleanup closed Installation writer", err)
	}
}

type completionCleanupContext struct {
	context.Context
	observe func()
}

func (ctx *completionCleanupContext) Err() error {
	if ctx.observe != nil {
		ctx.observe()
	}
	return ctx.Context.Err()
}

func TestInstallationNativeRecordedCompletionCleanupRetainsCancelledPrefix(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	stage := startBarrierFixture(t)
	base, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx := &completionCleanupContext{Context: base}
	barrier, err := prepareNativeStartBarrier(ctx, stage)
	stage.barrier = barrier
	if err != nil {
		t.Fatal(err)
	}
	// Observe an actual already removed socket, then cancel the ORIGINAL caller.
	// No successful ACK, manager or process is substituted by this control.
	ctx.observe = func() {
		if _, err := stage.lease.root.Lstat("start-completion.socket"); errors.Is(err, syscall.ENOENT) {
			cancel()
		}
	}
	if err := barrier.retireRecordedCompletion(); !errors.Is(err, context.Canceled) {
		t.Fatal("post-socket cancellation disappeared", err)
	}
	if barrier.cleaned || !barrier.cleanupAttempted || !errors.Is(barrier.terminal, context.Canceled) || barrier.listener != nil || barrier.connection != nil {
		t.Fatal("cancelled prefix became completed cleanup", barrier.terminal)
	}
	if err := observeStagedFile(stage.lease.root, "start-guard.json", barrier.guard); err != nil {
		t.Fatal("cancelled cleanup discarded original recovery guard", err)
	}
	if err := observeStagedFile(stage.lease.root, "start-socket.json", barrier.record); err != nil {
		t.Fatal("cancelled cleanup discarded original socket birth", err)
	}
	if len(stage.journal.Bytes(journal.Transitions, "completion-socket-removal.json")) == 0 ||
		len(stage.journal.Bytes(journal.Transitions, "completion-record-removal.json")) != 0 ||
		len(stage.journal.Bytes(journal.Transitions, "completion-guard-removal.json")) != 0 {
		t.Fatal("cancelled removal prefix lost provenance or recorded unattempted removals")
	}
	if _, err := stage.lease.writer.Stat(); err != nil {
		t.Fatal("cancelled cleanup released Installation writer", err)
	}
	if err := barrier.retireRecordedCompletion(); !errors.Is(err, ErrBinding) {
		t.Fatal("cancelled cleanup renewed its original caller", err)
	}
}

func TestInstallationNativeCompletionRemovalRechecksAfterJournalIO(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	stage := startBarrierFixture(t)
	ctx := &completionCleanupContext{Context: t.Context()}
	barrier, err := prepareNativeStartBarrier(ctx, stage)
	stage.barrier = barrier
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stage.lease.path, "start-completion.socket")
	var replacement *net.UnixListener
	var replaced os.FileInfo
	ctx.observe = func() {
		if replacement != nil || len(stage.journal.Bytes(journal.Transitions, "completion-socket-removal.json")) == 0 {
			return
		}
		// Keep the retired inode linked so immediate inode reuse cannot make
		// this substitution control indistinguishable from the original birth.
		if err := os.Rename(path, path+".retained-original"); err != nil {
			t.Fatal(err)
		}
		var err error
		replacement, err = net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
		if err != nil {
			t.Fatal(err)
		}
		replacement.SetUnlinkOnClose(false)
		t.Cleanup(func() { _ = replacement.Close() })
		if err := errors.Join(os.Chown(path, 0, int(barrier.record.gid)), os.Chmod(path, 0660)); err != nil {
			t.Fatal(err)
		}
		replaced, err = os.Lstat(path)
		if err != nil || os.SameFile(barrier.socket, replaced) {
			t.Fatal("substitution did not create a distinct socket", err)
		}
	}
	if err := barrier.retireRecordedCompletion(); !errors.Is(err, ErrBinding) || barrier.cleaned {
		t.Fatal("journal I/O admitted a substituted socket", err)
	}
	current, err := os.Lstat(path)
	if err != nil || replaced == nil || !os.SameFile(replaced, current) {
		t.Fatal("cleanup removed the replacement socket", err)
	}
	if err := observeStagedFile(stage.lease.root, "start-guard.json", barrier.guard); err != nil {
		t.Fatal("substitution discarded the recovery guard", err)
	}
}

func TestInstallationNativeCompletionSocketRejectsSameInodeAccessMutation(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	stage := startBarrierFixture(t)
	barrier, err := prepareNativeStartBarrier(t.Context(), stage)
	stage.barrier = barrier
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stage.lease.path, "start-completion.socket")
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0660); err != nil {
		t.Fatal(err)
	}
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(current, barrier.socket) ||
		current.Sys().(*syscall.Stat_t).Ctim == barrier.socket.Sys().(*syscall.Stat_t).Ctim {
		t.Fatal("same-inode mutation control did not change retained metadata", err)
	}
	if err := barrier.observeRecordedCompletion(); !errors.Is(err, ErrBinding) {
		t.Fatal("same-inode access mutation renewed the original socket", err)
	}
	if err := barrier.retireRecordedCompletion(); !errors.Is(err, ErrBinding) || barrier.cleanupAttempted {
		t.Fatal("changed original socket admitted removal", err)
	}
}

func TestInstallationNativeStartBarrierRetainsGuardAndExactSocketBirth(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	stage := startBarrierFixture(t)
	barrier, err := prepareNativeStartBarrier(t.Context(), stage)
	stage.barrier = barrier
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.observe(); err != nil {
		t.Fatal(err)
	}
	guard, err := stage.lease.root.ReadFile("start-guard.json")
	if err != nil || !bytes.Equal(guard, stage.intent.body) {
		t.Fatal("guard lost exact intent", err)
	}
	var record completionSocketBirth
	if err := json.Unmarshal(barrier.record.body, &record); err != nil {
		t.Fatal(err)
	}
	info, err := stage.lease.root.Lstat("start-completion.socket")
	if err != nil {
		t.Fatal(err)
	}
	native := info.Sys().(*syscall.Stat_t)
	if record.Schema != "ardents-endpoint-installation-start-socket-v1" || record.IntentDigest != digestHex(guard) || record.Device != uint64(native.Dev) || record.Inode != native.Ino || record.GID != 65534 || native.Uid != 0 || native.Gid != 65534 || info.Mode() != os.ModeSocket|0660 {
		t.Fatal("socket birth/access does not bind original intent")
	}
	if err := barrier.close(); err != nil {
		t.Fatal(err)
	}
	if err := observeStagedFile(stage.lease.root, "start-guard.json", barrier.guard); err != nil {
		t.Fatal("close removed guard", err)
	}
	if err := observeStagedFile(stage.lease.root, "start-socket.json", barrier.record); err != nil {
		t.Fatal("close removed record", err)
	}
	after, err := stage.lease.root.Lstat("start-completion.socket")
	if err != nil || !os.SameFile(info, after) {
		t.Fatal("close unlinked original socket", err)
	}
	if err := barrier.observe(); !errors.Is(err, ErrBinding) {
		t.Fatal("closed listener became active barrier", err)
	}
}

func TestInstallationNativeStartAckRefusesDetachedInvocationAndStoppedRecords(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	stage := startBarrierFixture(t)
	barrier, err := prepareNativeStartBarrier(t.Context(), stage)
	stage.barrier = barrier
	if err != nil {
		t.Fatal(err)
	}
	client, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: filepath.Join(stage.lease.path, "start-completion.socket"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	server, err := barrier.listener.AcceptUnix()
	if err != nil {
		client.Close()
		t.Fatal(err)
	}
	barrier.connection = server
	// Detached facts are deliberately not an accepted process, retained
	// transaction or manager observation. No fake positive ACK is supplied.
	pin := &installedProcessPin{pid: uint32(os.Getpid()), invocation: [16]byte{1}, previous: inspectedGeneration{selected: stage.selected}}
	if err := barrier.acknowledgeArchivedStart(stage, pin); !errors.Is(err, ErrBinding) {
		t.Fatal("detached invocation and stopped records authorized ACK", err)
	}
	if barrier.ackAttempted || barrier.ackWritten || stage.archivedIntent {
		t.Fatal("refusal introduced completion state")
	}
	if len(stage.journal.Bytes(journal.Transitions, "started-invocation.json")) != 0 {
		t.Fatal("refusal fabricated a start observation")
	}
	if err := barrier.close(); err != nil {
		t.Fatal(err)
	}
	var first [1]byte
	if n, err := client.Read(first[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatal("ACK bytes preceded actual retained admission", n, err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := observeStagedFile(stage.lease.root, "start-guard.json", barrier.guard); err != nil {
		t.Fatal("refusal discarded explicit recovery guard", err)
	}
}

func TestInstallationNativeStartBarrierRefusesResidueBeforeGuardBirth(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	for _, name := range []string{"start-guard.json", "start-socket.json", "start-completion.socket"} {
		t.Run(name, func(t *testing.T) {
			stage := startBarrierFixture(t)
			if err := stage.lease.root.WriteFile(name, []byte("retained foreign residue"), 0600); err != nil {
				t.Fatal(err)
			}
			barrier, err := prepareNativeStartBarrier(t.Context(), stage)
			if !errors.Is(err, ErrBinding) || barrier != nil {
				t.Fatal("residue adopted", err)
			}
			actual, err := stage.lease.root.ReadFile(name)
			if err != nil || string(actual) != "retained foreign residue" {
				t.Fatal("refusal changed residue", err)
			}
			if name != "start-guard.json" {
				if _, err := stage.lease.root.Lstat("start-guard.json"); !os.IsNotExist(err) {
					t.Fatal("residue refusal created guard", err)
				}
			}
		})
	}
}

func TestInstallationNativeStartBarrierRefusesSameAccessSocketReplacement(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	stage := startBarrierFixture(t)
	barrier, err := prepareNativeStartBarrier(t.Context(), stage)
	stage.barrier = barrier
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stage.lease.path, "start-completion.socket")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	foreign, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	foreign.SetUnlinkOnClose(false)
	t.Cleanup(func() { foreign.Close() })
	if err := os.Chown(path, 0, 65534); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0660); err != nil {
		t.Fatal(err)
	}
	foreignInfo, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := barrier.observe(); !errors.Is(err, ErrBinding) {
		t.Fatal("same-access replacement socket admitted", err)
	}
	if err := barrier.retireRecordedCompletion(); !errors.Is(err, ErrBinding) || barrier.cleanupAttempted {
		t.Fatal("cleanup adopted same-access replacement socket", err)
	}
	if err := observeStagedFile(stage.lease.root, "start-guard.json", barrier.guard); err != nil {
		t.Fatal("replacement refusal discarded original guard", err)
	}
	if err := observeStagedFile(stage.lease.root, "start-socket.json", barrier.record); err != nil {
		t.Fatal("replacement refusal discarded original birth record", err)
	}
	if err := barrier.close(); err != nil {
		t.Fatal(err)
	}
	remaining, err := os.Lstat(path)
	if err != nil || !os.SameFile(foreignInfo, remaining) {
		t.Fatal("original listener close removed foreign socket", err)
	}
}

func TestInstallationNativeStartBarrierCancellationRetainsGuard(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	stage := startBarrierFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	guardPath := filepath.Join(stage.lease.path, "start-guard.json")
	barrier, err := prepareNativeStartBarrier(&cancellationAtCreationRecord{Context: ctx, cancel: cancel, filename: guardPath}, stage)
	stage.barrier = barrier
	if !errors.Is(err, context.Canceled) || barrier == nil || !barrier.guardAttempted || barrier.listener != nil {
		t.Fatal("guard cancellation lost refusal/custody", err)
	}
	if err := observeStagedFile(stage.lease.root, "start-guard.json", barrier.guard); err != nil {
		t.Fatal("cancelled guard disappeared", err)
	}
	if _, err := stage.lease.root.Lstat("start-completion.socket"); !os.IsNotExist(err) {
		t.Fatal("cancelled guard created listener", err)
	}
}

func TestInstallationNativeStartBarrierRefusesClosedOriginalListener(t *testing.T) {
	t.Parallel() // Own private filesystem/Unix lifetime; no shared manager effects.
	stage := startBarrierFixture(t)
	barrier, err := prepareNativeStartBarrier(t.Context(), stage)
	stage.barrier = barrier
	if err != nil {
		t.Fatal(err)
	}
	if err := barrier.listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := barrier.observe(); !errors.Is(err, ErrBinding) {
		t.Fatal("closed original descriptor became a live barrier", err)
	}
	if err := observeStagedFile(stage.lease.root, "start-guard.json", barrier.guard); err != nil {
		t.Fatal("descriptor close erased guard", err)
	}
}

func TestInstallationNativeCompletionFramePreservesExactClosedBytes(t *testing.T) {
	selected := generationSelection{Schema: "ardents-endpoint-installation-selection-v1", GenerationDigest: strings.Repeat("a", 64), BindingDigest: strings.Repeat("b", 64)}
	invocation := [16]byte{0: 1, 15: 255}
	actual, err := installationCompletionFrame(selected, invocation)
	wanted := []byte("010000000000000000000000000000ff" + strings.Repeat("a", 64) + strings.Repeat("b", 64))
	if err != nil || len(actual) != 160 || !bytes.Equal(actual, wanted) {
		t.Fatal("completion grammar differs", err)
	}
	actual[0] = 'x'
	fresh, err := installationCompletionFrame(selected, invocation)
	if err != nil || !bytes.Equal(fresh, wanted) {
		t.Fatal("caller mutation changed exact frame", err)
	}
	for _, change := range []string{"schema", "generation", "binding", "uuid"} {
		other, uuid := selected, invocation
		switch change {
		case "schema":
			other.Schema = "foreign"
		case "generation":
			other.GenerationDigest = strings.Repeat("A", 64)
		case "binding":
			other.BindingDigest = "short"
		case "uuid":
			uuid = [16]byte{}
		}
		if _, err := installationCompletionFrame(other, uuid); !errors.Is(err, ErrBinding) {
			t.Fatal("invalid completion binding accepted", change, err)
		}
	}
}
