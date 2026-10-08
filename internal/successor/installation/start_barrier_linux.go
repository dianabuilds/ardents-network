package installation

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// A private refusal barrier, not evidence of an accepted running invocation.
// Its physical listener and original filesystem records outlive cancellation.
type installedStartBarrier struct {
	ctx              context.Context
	lease            *installationLease
	journal          *journal.Transition
	guard, record    stagedFile
	socket           os.FileInfo
	listener         *net.UnixListener
	guardAttempted   bool
	terminal         error
	connection       *net.UnixConn
	peerAttempted    bool
	owner            *successorPreparation
	accepted         *installedProcessPin
	ackAttempted     bool
	ackWritten       bool
	cleanupAttempted bool
	cleaned          bool
	startRecorded    bool
}

// This is durable intention, not a manager result or permission to replay Start.
// Its absence in an older journal does not establish that no Start occurred.
type candidateStartAttempt struct {
	Schema             string `json:"schema"`
	IntentDigest       string `json:"intent_digest"`
	SocketRecordDigest string `json:"socket_record_digest"`
}

func (barrier *installedStartBarrier) recordStartAttempt() (returnedErr error) {
	if barrier == nil || barrier.ctx == nil || barrier.journal == nil || barrier.startRecorded {
		return ErrBinding
	}
	defer func() { barrier.terminal = errors.Join(barrier.terminal, returnedErr) }()
	if err := barrier.ctx.Err(); err != nil {
		return err
	}
	if err := barrier.observe(); err != nil {
		return err
	}
	body, err := canonicalJSON(candidateStartAttempt{
		Schema:       "ardents-endpoint-installation-start-attempt-v1",
		IntentDigest: digestHex(barrier.guard.body), SocketRecordDigest: digestHex(barrier.record.body),
	})
	if err != nil {
		return err
	}
	if err := barrier.journal.Write(barrier.ctx, journal.Transitions, "start-attempt.json", body); err != nil {
		return err
	}
	// Journal I/O cannot renew a substituted original socket or guard.
	if err := barrier.observe(); err != nil {
		return err
	}
	barrier.startRecorded = true
	return barrier.ctx.Err()
}

// Compatibility identity/field order comes from the current installation owner.
// This record binds only the original socket birth to the exact guarded intent.
type completionSocketBirth struct {
	Schema       string `json:"schema"`
	IntentDigest string `json:"intent_digest"`
	Device       uint64 `json:"device"`
	Inode        uint64 `json:"inode"`
	GID          uint32 `json:"gid"`
}

// Native birth mechanism. Only its genuine retained owner supplies admission;
// this method itself can neither start a unit nor send completion bytes.
func prepareNativeStartBarrier(ctx context.Context, stage *generationStage) (result *installedStartBarrier, returnedErr error) {
	if ctx == nil || stage == nil || stage.lease == nil || stage.barrier != nil {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := stage.verifyTransitionPhase("0007.json", "successor-reloaded-stopped"); err != nil {
		return nil, err
	}
	if err := stage.observe(); err != nil {
		return nil, err
	}
	selectedBytes, err := canonicalJSON(stage.selected)
	selectedFile := stage.fixed[filepath.Join(stage.lease.path, "selection.json")]
	if err != nil || !bytes.Equal(selectedFile.file.body, selectedBytes) {
		return nil, errors.Join(ErrBinding, err)
	}
	var intent successorTransitionIntent
	if err := decodeCanonical(stage.intent.body, 128<<10, &intent); err != nil || intent.Schema != "ardents-endpoint-installation-successor-v1" || intent.Candidate != stage.selected || intent.Request.InstallationRoot != stage.lease.path {
		return nil, errors.Join(ErrBinding, err)
	}
	native, ok := stage.lease.identity.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid == 0 || intent.CandidateBinding.GID != native.Gid || stage.lease.identity.Mode() != os.ModeDir|0750 {
		return nil, ErrBinding
	}
	for _, name := range []string{"start-guard.json", "start-socket.json", "start-completion.socket"} {
		if _, err := stage.lease.root.Lstat(name); !errors.Is(err, os.ErrNotExist) {
			return nil, errors.Join(ErrBinding, err)
		}
	}
	barrier := &installedStartBarrier{ctx: ctx, lease: stage.lease, journal: stage.journal}
	defer func() {
		if returnedErr != nil {
			barrier.terminal = returnedErr
			if barrier.guardAttempted {
				result = barrier
			}
		}
	}()
	barrier.guardAttempted = true
	barrier.guard, err = writeStagedFile(ctx, stage.lease.root, "start-guard.json", stage.intent.body, 0600, 0)
	if err != nil {
		return nil, err
	}
	if err := syncStagingRoot(stage.lease.root); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := stage.lease.observe(); err != nil {
		return nil, err
	}
	if err := observeStagedFile(stage.lease.root, "start-guard.json", barrier.guard); err != nil {
		return nil, err
	}
	path := filepath.Join(stage.lease.path, "start-completion.socket")
	barrier.listener, err = net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	barrier.listener.SetUnlinkOnClose(false)
	birth, err := stage.lease.root.Lstat("start-completion.socket")
	if err != nil || birth.Mode()&os.ModeSocket == 0 {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := os.Lchown(path, 0, int(native.Gid)); err != nil {
		return nil, err
	}
	current, err := stage.lease.root.Lstat("start-completion.socket")
	if err != nil || !os.SameFile(birth, current) || current.Mode()&os.ModeSocket == 0 {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := os.Chmod(path, 0660); err != nil {
		return nil, err
	}
	barrier.socket, err = stage.lease.root.Lstat("start-completion.socket")
	if err != nil || !os.SameFile(birth, barrier.socket) || !completionSocketMatches(barrier.socket, barrier.socket, native.Gid) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := syncStagingRoot(stage.lease.root); err != nil {
		return nil, err
	}
	socketNative := barrier.socket.Sys().(*syscall.Stat_t)
	body, err := canonicalJSON(completionSocketBirth{Schema: "ardents-endpoint-installation-start-socket-v1", IntentDigest: digestHex(barrier.guard.body), Device: uint64(socketNative.Dev), Inode: socketNative.Ino, GID: native.Gid})
	if err != nil {
		return nil, err
	}
	barrier.record, err = writeStagedFile(ctx, stage.lease.root, "start-socket.json", body, 0600, 0)
	if err != nil {
		return nil, err
	}
	if err := syncStagingRoot(stage.lease.root); err != nil {
		return nil, err
	}
	if err := barrier.observe(); err != nil {
		return nil, err
	}
	return barrier, ctx.Err()
}

func completionSocketMatches(original, current os.FileInfo, gid uint32) bool {
	if original == nil || current == nil || !os.SameFile(original, current) || current.Mode() != os.ModeSocket|0660 {
		return false
	}
	native, ok := current.Sys().(*syscall.Stat_t)
	before, originalNative := original.Sys().(*syscall.Stat_t)
	return ok && originalNative && native.Dev != 0 && native.Ino != 0 && native.Uid == 0 && native.Gid == gid && native.Nlink == 1 &&
		native.Ctim == before.Ctim && native.Mtim == before.Mtim
}

func (barrier *installedStartBarrier) observe() error {
	if barrier == nil {
		return ErrBinding
	}
	if barrier.cleaned {
		if !barrier.ackWritten || barrier.owner == nil || barrier.owner.stage == nil || barrier.owner.stage.barrier != barrier ||
			barrier.owner.started == nil || !barrier.owner.started.acknowledged() {
			return ErrBinding
		}
		return errors.Join(barrier.observeRetiredCompletion(), barrier.terminal)
	}
	if barrier.lease == nil || barrier.listener == nil || barrier.terminal != nil {
		return errors.Join(ErrBinding, barrier.terminal)
	}
	raw, err := barrier.listener.SyscallConn()
	if err != nil {
		return errors.Join(ErrBinding, err)
	}
	var listenErr error
	if err := raw.Control(func(fd uintptr) {
		kind, kindErr := syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_TYPE)
		accepting, acceptErr := syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_ACCEPTCONN)
		if kindErr != nil || acceptErr != nil || kind != syscall.SOCK_STREAM || accepting != 1 {
			listenErr = errors.Join(ErrBinding, kindErr, acceptErr)
		}
	}); err != nil || listenErr != nil {
		return errors.Join(ErrBinding, err, listenErr)
	}
	return barrier.observeRecordedCompletion()
}

// Original filesystem provenance remains observable after physical socket Close.
// It is not start/ACK authority; the root transaction admits any removal.
func (barrier *installedStartBarrier) observeRecordedCompletion() error {
	if barrier == nil || barrier.lease == nil {
		return ErrBinding
	}
	if err := barrier.lease.observe(); err != nil {
		return err
	}
	if err := observeStagedFile(barrier.lease.root, "start-guard.json", barrier.guard); err != nil {
		return err
	}
	if err := observeStagedFile(barrier.lease.root, "start-socket.json", barrier.record); err != nil {
		return err
	}
	var record completionSocketBirth
	if err := decodeCanonical(barrier.record.body, 4<<10, &record); err != nil || record.Schema != "ardents-endpoint-installation-start-socket-v1" || record.IntentDigest != digestHex(barrier.guard.body) {
		return errors.Join(ErrBinding, err)
	}
	current, err := barrier.lease.root.Lstat("start-completion.socket")
	if err != nil || !completionSocketMatches(barrier.socket, current, record.GID) {
		return errors.Join(ErrBinding, err)
	}
	native := current.Sys().(*syscall.Stat_t)
	if uint64(native.Dev) != record.Device || native.Ino != record.Inode {
		return ErrBinding
	}
	return barrier.lease.observe()
}

// Only the original full-ACK owner may retire the recorded completion objects.
// A failed cleanup retains the irreversible ACK fact and original provenance;
// it grants no second ACK, process stop or runtime readiness.
func (barrier *installedStartBarrier) cleanupArchivedStart(stage *generationStage, pin *installedProcessPin) error {
	if barrier == nil || stage == nil || pin == nil || barrier.owner == nil || barrier.owner.stage != stage ||
		stage.barrier != barrier || barrier.accepted != pin || !barrier.ackAttempted || !barrier.ackWritten ||
		!stage.archivedIntent || barrier.owner.started == nil || barrier.owner.started.process != pin ||
		!barrier.owner.started.acknowledged() || barrier.cleanupAttempted {
		return ErrBinding
	}
	if err := barrier.owner.observeAcceptedCandidate(pin); err != nil {
		return err
	}
	started, err := observedCandidateStart(pin)
	if err != nil || !bytes.Equal(stage.journal.Bytes(journal.Transitions, "started-invocation.json"), started) {
		return errors.Join(ErrBinding, err)
	}
	if err := stage.observeIntent(); err != nil {
		return err
	}
	return barrier.retireRecordedCompletion()
}

// Physical provenance cleanup only. Filesystem fixtures may exercise it without
// minting a full ACK or an accepting transition. The product caller above owns
// admission; this lifetime never renews its original caller or retries a prefix.
func (barrier *installedStartBarrier) retireRecordedCompletion() (returnedErr error) {
	if barrier == nil || barrier.ctx == nil || barrier.cleanupAttempted || barrier.cleaned {
		return ErrBinding
	}
	if err := barrier.ctx.Err(); err != nil {
		return err
	}
	if err := barrier.observeRecordedCompletion(); err != nil {
		return err
	}
	barrier.cleanupAttempted = true
	defer func() {
		returnedErr = errors.Join(returnedErr, barrier.ctx.Err())
		barrier.terminal = errors.Join(barrier.terminal, returnedErr)
	}()
	// All original read/write cancellation callbacks have joined before the
	// admitted product caller reaches here. Close precedes any pathname unlink.
	if err := barrier.close(); err != nil {
		return err
	}
	if err := barrier.observeRecordedCompletion(); err != nil {
		return err
	}
	// Keep the guard until the recorded socket and its birth record have each
	// been removed and the original parent directory has been synchronized.
	names := []string{"start-completion.socket", "start-socket.json", "start-guard.json"}
	for index, name := range names {
		if err := barrier.ctx.Err(); err != nil {
			return err
		}
		if err := barrier.lease.observe(); err != nil {
			return err
		}
		for _, retired := range names[:index] {
			if _, err := barrier.lease.root.Lstat(retired); !errors.Is(err, syscall.ENOENT) {
				return errors.Join(ErrBinding, err)
			}
		}
		switch name {
		case "start-completion.socket":
			if err := barrier.observeRecordedCompletion(); err != nil {
				return err
			}
		case "start-socket.json":
			if err := observeStagedFile(barrier.lease.root, name, barrier.record); err != nil {
				return err
			}
			if err := observeStagedFile(barrier.lease.root, "start-guard.json", barrier.guard); err != nil {
				return err
			}
		case "start-guard.json":
			if err := observeStagedFile(barrier.lease.root, name, barrier.guard); err != nil {
				return err
			}
		}
		// Preserve original provenance durably before losing the pathname. A
		// recovery opener must not infer an authorized removal from absence.
		if err := barrier.recordCompletionRemoval(index, name); err != nil {
			return err
		}
		if err := barrier.lease.root.Remove(name); err != nil {
			return err
		}
		if err := syncStagingRoot(barrier.lease.root); err != nil {
			return err
		}
	}
	if err := barrier.observeRetiredCompletion(); err != nil {
		return err
	}
	barrier.cleaned = true
	return nil
}

// This is an intended physical removal, never evidence of ACK delivery. The
// admitted product caller retains its process/archival checks independently.
type completionRemoval struct {
	Schema       string `json:"schema"`
	IntentDigest string `json:"intent_digest"`
	Name         string `json:"name"`
	Device       uint64 `json:"device"`
	Inode        uint64 `json:"inode"`
	UID          uint32 `json:"uid"`
	GID          uint32 `json:"gid"`
	Mode         uint32 `json:"mode"`
	Size         int64  `json:"size"`
	ChangedSec   int64  `json:"changed_sec"`
	ChangedNSec  int64  `json:"changed_nsec"`
	ModifiedSec  int64  `json:"modified_sec"`
	ModifiedNSec int64  `json:"modified_nsec"`
	BytesDigest  string `json:"bytes_digest"`
}

func (barrier *installedStartBarrier) recordCompletionRemoval(index int, name string) error {
	if barrier == nil || barrier.journal == nil || !barrier.cleanupAttempted ||
		barrier.listener != nil || barrier.connection != nil {
		return ErrBinding
	}
	names := []string{"start-completion.socket", "start-socket.json", "start-guard.json"}
	records := []string{"completion-socket-removal.json", "completion-record-removal.json", "completion-guard-removal.json"}
	if index < 0 || index >= len(names) || name != names[index] {
		return ErrBinding
	}
	identity, digest := barrier.socket, ""
	switch index {
	case 1:
		identity, digest = barrier.record.identity, digestHex(barrier.record.body)
	case 2:
		identity, digest = barrier.guard.identity, digestHex(barrier.guard.body)
	}
	if identity == nil {
		return ErrBinding
	}
	native, ok := identity.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Dev == 0 || native.Ino == 0 {
		return ErrBinding
	}
	body, err := canonicalJSON(completionRemoval{
		Schema:       "ardents-endpoint-installation-completion-removal-v1",
		IntentDigest: digestHex(barrier.guard.body), Name: name,
		Device: uint64(native.Dev), Inode: native.Ino, UID: native.Uid,
		GID: native.Gid, Mode: native.Mode, BytesDigest: digest,
		Size: native.Size, ChangedSec: native.Ctim.Sec, ChangedNSec: native.Ctim.Nsec,
		ModifiedSec: native.Mtim.Sec, ModifiedNSec: native.Mtim.Nsec,
	})
	if err != nil {
		return err
	}
	if err := barrier.journal.Write(barrier.ctx, journal.Transitions, records[index], body); err != nil {
		return err
	}
	// Journal I/O may block while an object is replaced. Check the original
	// again after the durable record and immediately before pathname removal.
	if err := barrier.lease.observe(); err != nil {
		return err
	}
	if index == 0 {
		return barrier.observeRecordedCompletion()
	}
	file := barrier.record
	if index == 2 {
		file = barrier.guard
	}
	return errors.Join(observeStagedFile(barrier.lease.root, name, file), barrier.ctx.Err())
}

func (barrier *installedStartBarrier) observeRetiredCompletion() error {
	if barrier == nil || barrier.lease == nil || barrier.listener != nil || barrier.connection != nil || !barrier.cleanupAttempted {
		return ErrBinding
	}
	if err := barrier.lease.observe(); err != nil {
		return err
	}
	for _, name := range []string{"start-completion.socket", "start-socket.json", "start-guard.json"} {
		if _, err := barrier.lease.root.Lstat(name); !errors.Is(err, syscall.ENOENT) {
			return errors.Join(ErrBinding, err)
		}
	}
	return barrier.lease.observe()
}

// Physical close preserves guard/socket records and the original socket path.
// Only later authenticated completion/recovery may remove recorded provenance.
func (barrier *installedStartBarrier) close() error {
	if barrier == nil {
		return nil
	}
	if barrier.connection != nil {
		barrier.terminal = errors.Join(barrier.terminal, barrier.connection.Close())
		barrier.connection = nil
	}
	if barrier.listener != nil {
		barrier.terminal = errors.Join(barrier.terminal, barrier.listener.Close())
		barrier.listener = nil
	}
	return barrier.terminal
}

// This method observes a live candidate, but sends no ACK. Its retained process
// and original caller must survive every native observation and bounded read.
func (barrier *installedStartBarrier) acceptCompletion(pin *installedProcessPin) (returnedErr error) {
	if barrier == nil || barrier.ctx == nil || pin == nil || pin.ctx == nil || barrier.peerAttempted || barrier.connection != nil {
		return ErrInput
	}
	deadline, bounded := barrier.ctx.Deadline()
	if !bounded {
		return ErrInput
	}
	if originalDeadline, bounded := pin.ctx.Deadline(); bounded && originalDeadline.Before(deadline) {
		deadline = originalDeadline
	}
	if err := barrier.observeCompletionInvocation(pin); err != nil {
		return err
	}
	barrier.peerAttempted = true
	defer func() {
		returnedErr = errors.Join(returnedErr, barrier.ctx.Err(), pin.ctx.Err())
		if returnedErr != nil {
			barrier.accepted = nil
			barrier.terminal = errors.Join(barrier.terminal, returnedErr)
			if barrier.connection != nil {
				closeErr := barrier.connection.Close()
				if !errors.Is(closeErr, net.ErrClosed) {
					returnedErr = errors.Join(returnedErr, closeErr)
				}
				barrier.connection = nil
			}
		}
	}()
	if err := barrier.listener.SetDeadline(deadline); err != nil {
		return err
	}
	listenerClosed := make(chan error, 1)
	stopListener := context.AfterFunc(barrier.ctx, func() { listenerClosed <- barrier.listener.Close() })
	defer func() {
		if !stopListener() {
			returnedErr = errors.Join(returnedErr, <-listenerClosed)
		}
	}()
	originalListenerClosed := make(chan error, 1)
	stopOriginalListener := context.AfterFunc(pin.ctx, func() { originalListenerClosed <- barrier.listener.Close() })
	defer func() {
		if !stopOriginalListener() {
			returnedErr = errors.Join(returnedErr, <-originalListenerClosed)
		}
	}()
	connection, err := barrier.listener.AcceptUnix()
	if err != nil {
		return err
	}
	barrier.connection = connection
	if err := connection.SetDeadline(deadline); err != nil {
		return err
	}
	connectionClosed := make(chan error, 1)
	stopConnection := context.AfterFunc(barrier.ctx, func() { connectionClosed <- connection.Close() })
	defer func() {
		if !stopConnection() {
			returnedErr = errors.Join(returnedErr, <-connectionClosed)
		}
	}()
	originalConnectionClosed := make(chan error, 1)
	stopOriginalConnection := context.AfterFunc(pin.ctx, func() { originalConnectionClosed <- connection.Close() })
	defer func() {
		if !stopOriginalConnection() {
			returnedErr = errors.Join(returnedErr, <-originalConnectionClosed)
		}
	}()
	if err := barrier.observeCompletionInvocation(pin); err != nil {
		return err
	}
	if err := readInstallationCompletionPeer(connection, pin.pid, pin.previous.binding.UID, pin.previous.binding.GID, pin.previous.selected, pin.invocation); err != nil {
		return err
	}
	if err := barrier.observeCompletionInvocation(pin); err != nil {
		return err
	}
	barrier.accepted = pin
	return nil
}

// Only the original transaction's actual accepted process/peer and durable
// archival permit this send. Detached records grant no startup authority.
func (barrier *installedStartBarrier) acknowledgeArchivedStart(stage *generationStage, pin *installedProcessPin) (returnedErr error) {
	if barrier == nil || stage == nil || pin == nil || barrier.owner == nil ||
		barrier.owner.stage != stage || stage.barrier != barrier || barrier.accepted != pin ||
		barrier.connection == nil || barrier.ackAttempted || !stage.archivedIntent {
		return ErrBinding
	}
	if err := barrier.owner.observeAcceptedCandidate(pin); err != nil {
		return err
	}
	started, err := observedCandidateStart(pin)
	if err != nil || !bytes.Equal(stage.journal.Bytes(journal.Transitions, "started-invocation.json"), started) {
		return errors.Join(ErrBinding, err)
	}
	if err := stage.observeIntent(); err != nil {
		return err
	}
	barrier.ackAttempted = true
	defer func() {
		returnedErr = errors.Join(returnedErr, barrier.ctx.Err(), pin.ctx.Err())
		if returnedErr != nil {
			barrier.terminal = errors.Join(barrier.terminal, returnedErr)
		}
	}()
	connection := barrier.connection
	closed := make(chan error, 1)
	stop := context.AfterFunc(barrier.ctx, func() { closed <- connection.Close() })
	defer func() {
		if !stop() {
			err := <-closed
			if !errors.Is(err, net.ErrClosed) {
				returnedErr = errors.Join(returnedErr, err)
			}
		}
	}()
	originalClosed := make(chan error, 1)
	stopOriginal := context.AfterFunc(pin.ctx, func() { originalClosed <- connection.Close() })
	defer func() {
		if !stopOriginal() {
			err := <-originalClosed
			if !errors.Is(err, net.ErrClosed) {
				returnedErr = errors.Join(returnedErr, err)
			}
		}
	}()
	frame, err := installationCompletionFrame(pin.previous.selected, pin.invocation)
	if err != nil {
		return err
	}
	if err := barrier.owner.observeAcceptedCandidate(pin); err != nil {
		return err
	}
	count, err := io.Copy(connection, bytes.NewReader(frame))
	// A full frame may be visible despite a late cancellation/cleanup error.
	// Preserve that fact for the eventual post-acceptance outcome; no ACK retry
	// or reclassification as a never-admitted invocation is permitted.
	if count == int64(len(frame)) {
		barrier.ackWritten = true
	}
	if err != nil || count != int64(len(frame)) {
		return errors.Join(io.ErrShortWrite, err)
	}
	return errors.Join(barrier.ctx.Err(), pin.ctx.Err())
}

func (barrier *installedStartBarrier) observeCompletionInvocation(pin *installedProcessPin) error {
	if barrier == nil || barrier.ctx == nil || pin == nil || pin.ctx == nil || pin.reader == nil || pin.reader.lease != barrier.lease {
		return ErrBinding
	}
	if err := barrier.ctx.Err(); err != nil {
		return err
	}
	if err := pin.ctx.Err(); err != nil {
		return err
	}
	if err := barrier.observe(); err != nil {
		return err
	}
	var intent successorTransitionIntent
	if err := decodeCanonical(barrier.guard.body, 128<<10, &intent); err != nil || intent.Schema != "ardents-endpoint-installation-successor-v1" || intent.Candidate != pin.previous.selected {
		return errors.Join(ErrBinding, err)
	}
	binding, err := canonicalJSON(pin.previous.binding)
	wanted, wantedErr := canonicalJSON(intent.CandidateBinding)
	if err != nil || wantedErr != nil || !bytes.Equal(binding, wanted) {
		return errors.Join(ErrBinding, err, wantedErr)
	}
	version, err := systemd.Version(barrier.ctx)
	if err != nil {
		return err
	}
	unit, service, err := systemd.Endpoint(barrier.ctx)
	if err != nil {
		return err
	}
	if err := verifyRunningEndpointProperties(version, unit, service, pin.previous.request, pin.previous.selected.GenerationDigest, pin.pid, pin.invocation); err != nil {
		return err
	}
	if err := pin.observe(); err != nil {
		return err
	}
	if err := observeBoundActivationSockets(barrier.ctx); err != nil {
		return err
	}
	unit, service, err = systemd.Endpoint(barrier.ctx)
	if err != nil {
		return err
	}
	if err := verifyRunningEndpointProperties(version, unit, service, pin.previous.request, pin.previous.selected.GenerationDigest, pin.pid, pin.invocation); err != nil {
		return err
	}
	if err := pin.observe(); err != nil {
		return err
	}
	return errors.Join(barrier.ctx.Err(), pin.ctx.Err())
}

// Native SO_PEERCRED and byte mechanism. Supplied facts here grant no manager,
// process, Release or startup admission; the owning method proves them above.
func readInstallationCompletionPeer(connection *net.UnixConn, pid, uid, gid uint32, selected generationSelection, invocation [16]byte) error {
	if connection == nil || pid == 0 {
		return ErrInput
	}
	wanted, err := installationCompletionFrame(selected, invocation)
	if err != nil {
		return err
	}
	raw, err := connection.SyscallConn()
	if err != nil {
		return err
	}
	var peer *syscall.Ucred
	var peerErr error
	if err := raw.Control(func(fd uintptr) {
		peer, peerErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil || peerErr != nil || peer == nil || peer.Pid <= 0 || uint32(peer.Pid) != pid || peer.Uid != uid || peer.Gid != gid {
		return errors.Join(ErrBinding, err, peerErr)
	}
	actual := make([]byte, len(wanted))
	if _, err := io.ReadFull(connection, actual); err != nil {
		return err
	}
	if !bytes.Equal(actual, wanted) {
		return ErrBinding
	}
	return nil
}

// Installation admits selection identity; completion owns the shared frame
// grammar consumed by both the root producer and the independent startup peer.
func installationCompletionFrame(selected generationSelection, invocation [16]byte) ([]byte, error) {
	if selected.Schema != "ardents-endpoint-installation-selection-v1" {
		return nil, ErrBinding
	}
	frame, err := completion.Encode(invocation, selected.GenerationDigest, selected.BindingDigest)
	if err != nil {
		return nil, errors.Join(ErrBinding, err)
	}
	return frame[:], nil
}
