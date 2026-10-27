package forwarding

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/process"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

type stopAttempt struct {
	ctx               context.Context
	started           Started
	identity          *runtimeIdentity
	namespace         *startNamespace
	manager           *systemd.ForwardingControl
	invocation        process.ForwardingInvocation
	scope             *cgroup.ForwardingScope
	storage           *retirementStorage
	attempted, joined bool
	terminal          error
	closed            bool
	closeErr          error
	phase             stopPhase
}

// Stop diagnostics expose only fixed stages. Original causes remain available
// to errors.Is/As without printing paths, manager facts or private histories.
type stopPhase uint8

const (
	stopPlatform stopPhase = iota
	stopHistory
	stopController
	stopIdentity
	stopManager
	stopManagerIdentity
	stopManagerFacts
	stopNamespace
	stopProcess
	stopScope
	stopHistoryCheck
	stopIdentityCheck
	stopNamespaceCheck
	stopProcessCheck
	stopScopeCheck
	stopRunningPolicy
	stopExecutionClock
	stopRetirement
)

type stopFailure struct {
	phase stopPhase
	cause error
}

func (e *stopFailure) Error() string {
	labels := [...]string{"platform", "history", "controller", "identity", "manager", "manager-identity", "manager-facts", "namespace", "process", "scope", "history-check", "identity-check", "namespace-check", "process-check", "scope-check", "running-policy", "execution-clock", "retirement"}
	label := "internal"
	if int(e.phase) < len(labels) {
		label = labels[e.phase]
	}
	return "forwarding stop: " + label + " refused"
}

func (e *stopFailure) Unwrap() error { return e.cause }

// Stop independently admits the latest invocation's explicit stop from retained
// history. Release expiry cannot prohibit shutdown. After any original pidfd
// signal attempt, custody remains held until the original kernel scope joins,
// even if the caller has expired. A missing domain report never permits restart.
func Stop(ctx context.Context, v *release.Verifier, bindingPath, history string) (returned error) {
	if ctx == nil || v == nil || os.Geteuid() != 0 || !validPath(bindingPath) || !validPath(history) {
		return ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return ErrInput
	}
	s := &stopAttempt{ctx: ctx}
	defer func() {
		phase := s.phase
		s.terminal = errors.Join(s.terminal, returned)
		if s.attempted && !s.joined {
			s.joinOriginal()
		}
		returned = errors.Join(s.terminal, s.close())
		if returned != nil {
			returned = &stopFailure{phase: phase, cause: returned}
		}
	}()
	if err := installedPlatform(ctx); err != nil {
		return err
	}
	var err error
	s.phase = stopHistory
	s.started, err = ReadStarted(ctx, v, bindingPath, history)
	if err != nil {
		return err
	}
	if s.started.original.latestRetirement != nil {
		return ErrBinding
	}
	c := s.started.original.completed.original
	b, a := c.binding, s.started.original.snapshot.archive
	program := c.files.files[filepath.Join(c.files.sealed, programName)]
	s.phase = stopController
	if err := verifyControllerProgram(ctx, program.body); err != nil {
		return errors.Join(errControllerProgram, err)
	}
	s.phase = stopIdentity
	s.identity, err = openRootIdentity(ctx)
	if err != nil {
		return err
	}
	s.phase = stopManager
	s.manager, err = systemd.OpenForwardingControl(ctx)
	if err != nil {
		return err
	}
	s.phase = stopManagerIdentity
	manager, err := s.manager.ManagerIdentity()
	if err != nil || manager != a.Manager {
		return errors.Join(ErrBinding, err)
	}
	s.phase = stopManagerFacts
	facts, err := s.manager.Observe()
	if err != nil {
		return err
	}
	s.phase = stopNamespace
	s.namespace, err = openStartNamespace(ctx, facts.UnitPaths)
	if err != nil {
		return err
	}
	s.phase = stopProcess
	s.invocation, err = process.RetainForwardingStop(ctx, process.ForwardingExpected{PID: a.PID, UID: b.UID, GID: b.GID, Invocation: s.started.original.snapshot.id, InstallationRoot: b.Request.InstallationRoot, Generation: b.Generation, ProgramIdentity: program.info}, a.ProcessDevice, a.ProcessInode)
	if err != nil {
		return err
	}
	s.phase = stopScope
	s.scope, err = cgroup.RetainForwardingStop(ctx, a.PID, a.CgroupDevice, a.CgroupInode)
	if err != nil {
		return err
	}
	if err := s.observeOriginal(); err != nil {
		return err
	}
	s.phase = stopRetirement
	s.storage, err = createRetirementStorage(ctx, b.Request.InstallationRoot, s.started.original.snapshot.number, s.started.original.snapshot.sha256)
	if err != nil {
		return err
	}
	if err := s.storage.birthDirectory(b.GID, s.observeOriginal); err != nil {
		return err
	}
	if _, err := s.storage.write("retirement-pending", []byte("ardents-forwarding-retirement-pending-v1\n"+s.started.original.snapshot.sha256+"\n"), s.observeOriginal); err != nil {
		return err
	}
	if err := s.storage.prepareSocket(s.observeOriginal); err != nil {
		return err
	}
	var challenge [32]byte
	if _, err := rand.Read(challenge[:]); err != nil || challenge == [32]byte{} {
		return errors.Join(ErrBinding, err)
	}
	if err := s.storage.record("interrupt-attempt", s.storage.path, nil, digest(challenge[:])); err != nil {
		return err
	}
	if err := s.observeOriginal(); err != nil {
		return err
	}
	s.attempted, err = s.invocation.InterruptForwarding(ctx)
	if err != nil {
		return err
	}
	frame, err := s.acceptCompletion(challenge)
	if err != nil {
		return err
	}
	if err := s.storage.record("domain-joined", s.storage.path, nil, digest(frame[:])); err != nil {
		return err
	}
	s.joinOriginal()
	if s.terminal != nil {
		return s.terminal
	}
	execution, err := s.observeCompleted()
	if err != nil {
		return err
	}
	r := retirementResult{Schema: "ardents-forwarding-retirement-v1", Attempt: uint32(s.started.original.snapshot.number), Generation: b.Generation, BindingSHA256: a.BindingSHA256, StartSHA256: s.started.original.snapshot.sha256, InvocationID: a.InvocationID, PID: a.PID, Manager: a.Manager, Challenge: hex.EncodeToString(challenge[:]), CompletionSHA256: digest(frame[:]), StartedRealtime: execution.startedRealtime, StartedMonotonic: execution.startedMonotonic, ExitedRealtime: execution.exitedRealtime, ExitedMonotonic: execution.exitedMonotonic, Code: execution.code, Status: execution.status}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := s.storage.record("native-joined", s.storage.path, nil, digest(raw)); err != nil {
		return err
	}
	if err := s.closeOriginal(); err != nil {
		return err
	}
	if err := s.storage.record("native-closed", s.storage.path, nil, digest(raw)); err != nil {
		return err
	}
	result, err := s.storage.write("retirement-result.json", raw, s.base)
	if err != nil {
		return err
	}
	if err := s.storage.record("retired", result.path, result.info, digest(raw)); err != nil {
		return err
	}
	// Reopen actual synced files with independent descriptors while the original
	// writer still excludes other root operations and its blocker remains.
	proof, err := readRetirementSnapshot(ctx, b, s.started.original.snapshot.number, a.BindingSHA256, s.started.original.snapshot.sha256, a, s.started.original.snapshot.id, retirementReadback)
	if err != nil {
		return errors.Join(err, proof.close())
	}
	checkErr := proof.check()
	if proof.sha256 != digest(raw) {
		checkErr = errors.Join(checkErr, ErrBinding)
	}
	if err := errors.Join(checkErr, proof.close()); err != nil {
		return err
	}
	if err := s.base(); err != nil {
		return err
	}
	if err := s.close(); err != nil {
		return err
	}
	return s.storage.publish()
}

func (s *stopAttempt) base() error {
	phase := s.phase
	if err := s.ctx.Err(); err != nil {
		return err
	}
	s.phase = stopHistoryCheck
	if err := s.started.checkWithEffects(nil, s.storage); err != nil {
		return err
	}
	s.phase = stopIdentityCheck
	if err := s.identity.check(); err != nil {
		return err
	}
	s.phase = stopNamespaceCheck
	if err := s.namespace.check(); err != nil {
		return err
	}
	if s.manager != nil {
		s.phase = stopManagerIdentity
		id, err := s.manager.ManagerIdentity()
		if err != nil || id != s.started.original.snapshot.archive.Manager {
			return errors.Join(ErrBinding, err)
		}
	}
	if s.storage != nil {
		s.phase = stopRetirement
		if err := s.storage.check(); err != nil {
			return err
		}
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	s.phase = phase
	return nil
}

func (s *stopAttempt) observeOriginal() error {
	phase := s.phase
	if s.scope == nil {
		return ErrBinding
	}
	b, a := s.started.original.completed.original.binding, s.started.original.snapshot.archive
	for range 2 {
		if err := s.base(); err != nil {
			return err
		}
		s.phase = stopProcessCheck
		if err := s.invocation.Observe(s.ctx); err != nil {
			return err
		}
		s.phase = stopScopeCheck
		if err := s.scope.Observe(); err != nil {
			return err
		}
		s.phase = stopManagerFacts
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		s.phase = stopRunningPolicy
		if err := verifyForwardingRunning(facts.Unit, facts.Service, b.Request, b.Generation, a.PID, s.started.original.snapshot.id); err != nil {
			return err
		}
		s.phase = stopExecutionClock
		execution, err := forwardingExecutionStatus(facts.Service)
		if err != nil || execution.startedRealtime != a.StartedRealtime || execution.startedMonotonic != a.StartedMonotonic {
			return errors.Join(ErrBinding, err)
		}
		s.phase = stopProcessCheck
		if err := s.invocation.Observe(s.ctx); err != nil {
			return err
		}
		s.phase = stopScopeCheck
		if err := s.scope.Observe(); err != nil {
			return err
		}
	}
	if err := s.base(); err != nil {
		return err
	}
	s.phase = phase
	return nil
}

func (s *stopAttempt) acceptCompletion(challenge [32]byte) (frame [retirementFrameSize]byte, returned error) {
	if err := s.base(); err != nil {
		return frame, err
	}
	deadline, ok := s.ctx.Deadline()
	if !ok || s.storage.listener == nil || challenge == [32]byte{} {
		return frame, ErrInput
	}
	listener := s.storage.listener
	if err := listener.SetDeadline(deadline); err != nil {
		return frame, err
	}
	closed := make(chan error, 1)
	stop := context.AfterFunc(s.ctx, func() { closed <- listener.Close() })
	defer func() {
		if !stop() {
			physical := <-closed
			if !errors.Is(physical, net.ErrClosed) {
				returned = errors.Join(returned, physical)
			}
		}
	}()
	connection, err := listener.AcceptUnix()
	if err != nil {
		return frame, err
	}
	defer func() { returned = errors.Join(returned, connection.Close(), s.ctx.Err()) }()
	connectionClosed := make(chan error, 1)
	stopConnection := context.AfterFunc(s.ctx, func() { connectionClosed <- connection.Close() })
	defer func() {
		if !stopConnection() {
			returned = errors.Join(returned, <-connectionClosed)
		}
	}()
	if err := connection.SetDeadline(deadline); err != nil {
		return frame, err
	}
	b, a := s.started.original.completed.original.binding, s.started.original.snapshot.archive
	peer, err := retirementPeer(connection)
	if err != nil || peer.Pid <= 0 || uint32(peer.Pid) != a.PID || peer.Uid != b.UID || peer.Gid != b.GID {
		return frame, errors.Join(ErrBinding, err)
	}
	if err := s.invocation.Observe(s.ctx); err != nil {
		return frame, err
	}
	if err := s.scope.Observe(); err != nil {
		return frame, err
	}
	if err := s.base(); err != nil {
		return frame, err
	}
	if n, err := io.Copy(connection, bytes.NewReader(challenge[:])); err != nil || n != int64(len(challenge)) {
		return frame, errors.Join(io.ErrShortWrite, err)
	}
	if _, err := io.ReadFull(connection, frame[:]); err != nil {
		return frame, err
	}
	start, err := completion.Encode(s.started.original.snapshot.id, b.Generation, a.BindingSHA256)
	if err != nil {
		return frame, err
	}
	if frame != retirementFrame(start, challenge) {
		return frame, ErrBinding
	}
	// The original reporter may already have exited after its complete write.
	// Requiring a still-live proc here would reject an honest joined report.
	return frame, s.base()
}

func (s *stopAttempt) joinOriginal() {
	if s.scope == nil {
		select {}
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	firstFailure := false
	for {
		joined, err := s.scope.Joined()
		if err != nil && !firstFailure {
			s.terminal = errors.Join(s.terminal, err)
			firstFailure = true
		}
		if joined {
			s.joined = true
			return
		}
		<-ticker.C
	}
}

func (s *stopAttempt) observeCompleted() (forwardingExecution, error) {
	if !s.joined {
		return forwardingExecution{}, ErrBinding
	}
	b, a := s.started.original.completed.original.binding, s.started.original.snapshot.archive
	var result forwardingExecution
	for range 2 {
		if err := s.base(); err != nil {
			return result, err
		}
		facts, err := s.manager.Observe()
		if err != nil {
			return result, err
		}
		execution, err := verifyForwardingExited(facts.Unit, facts.Service, b.Request, b.Generation, a, s.started.original.snapshot.id)
		if err != nil {
			return result, err
		}
		if result != (forwardingExecution{}) && result != execution {
			return result, ErrBinding
		}
		result = execution
		if err := s.base(); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *stopAttempt) closeOriginal() error {
	physical := s.storage.closeListener()
	physical = errors.Join(physical, s.manager.Close(), s.invocation.Close(), s.scope.Close())
	s.manager, s.scope = nil, nil
	s.invocation = process.ForwardingInvocation{}
	return physical
}

func (s *stopAttempt) close() error {
	if s == nil {
		return nil
	}
	if s.closed {
		return s.closeErr
	}
	s.closed = true
	// Native resources, namespace/kernel and writer close before returning the
	// independently held start/initial leases. No live object passes to restart.
	s.closeErr = errors.Join(s.manager.Close(), s.invocation.Close(), s.scope.Close(), s.namespace.close(), s.identity.close(), s.storage.close(), s.started.Close(), s.ctx.Err())
	return s.closeErr
}
