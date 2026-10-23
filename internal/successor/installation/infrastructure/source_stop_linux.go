package infrastructure

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

type sourceStop struct {
	ctx               context.Context
	chain             *sourceHistoryChain
	identity          *sourceRuntimeIdentity
	namespace         *sourceNamespace
	managerIdentity   string
	manager           *systemd.SourceControl
	invocation        *process.Invocation
	scope             *cgroup.SourceScope
	storage           *sourceStopStorage
	attempted, joined bool
	terminal          error
}

// StopSource admits one explicit original Source stop. It requires retained
// history identity, not unexpired Release authorization: expiry cannot prevent
// safe shutdown. Restart independently requires fresh Release admission. A
// failed/unknown original stop leaves its exclusive prefix as repair-required.
func StopSource(ctx context.Context, v *release.Verifier, binding, history string) (err error) {
	if ctx == nil || v == nil || os.Geteuid() != 0 {
		return ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return ErrInput
	}
	s := &sourceStop{ctx: ctx}
	defer func() {
		s.terminal = errors.Join(s.terminal, err)
		if s.attempted && !s.joined {
			s.joinOriginal()
		}
		err = errors.Join(s.terminal, s.close())
	}()
	if err = sourceInstalledPlatform(ctx); err != nil {
		return err
	}
	s.chain, err = openSourceHistoryChain(ctx, v, binding, history)
	if err != nil {
		return err
	}
	selection := s.chain.initial.selection
	if s.chain.latestSelection != selection {
		return ErrBinding
	}
	b := selection.binding
	started := s.chain.latest
	if err = verifySourceControllerProgram(ctx, selection.files[filepath.Join(selection.sealed, ProgramName)].raw); err != nil {
		return err
	}
	s.identity, err = openSourceRootIdentity(ctx)
	if err != nil {
		return err
	}
	s.managerIdentity, err = sourceManagerIdentity()
	if err != nil {
		return err
	}
	s.manager, err = systemd.OpenSourceControl(ctx)
	if err != nil {
		return err
	}
	facts, err := s.manager.Observe()
	if err != nil {
		return err
	}
	s.namespace, err = openSourceNamespace(ctx, facts.UnitPaths)
	if err != nil {
		return err
	}
	program := selection.files[filepath.Join(selection.sealed, ProgramName)]
	s.invocation, err = process.RetainSourceStop(ctx, process.SourceExpected{PID: started.archive.PID, UID: b.UID, GID: b.GID, Invocation: started.id, InstallationRoot: b.Request.InstallationRoot, Generation: b.Generation, ProgramIdentity: program.info}, started.archive.ProcessDevice, started.archive.ProcessInode)
	if err != nil {
		return err
	}
	s.scope, err = cgroup.RetainSourceStop(ctx, started.archive.PID, started.archive.CgroupDevice, started.archive.CgroupInode)
	if err != nil {
		return err
	}
	if err = s.observeOriginal(); err != nil {
		return err
	}
	s.storage, err = openSourceStopStorage(ctx, selection, started)
	if err != nil {
		return err
	}
	if err = s.storage.prepareSocket(s.observeOriginal); err != nil {
		return err
	}
	var challenge [32]byte
	if _, err = rand.Read(challenge[:]); err != nil || challenge == [32]byte{} {
		return errors.Join(ErrBinding, err)
	}
	if err = s.storage.record("interrupt-attempt", s.storage.directory, nil, digest(challenge[:])); err != nil {
		return err
	}
	if err = s.observeOriginal(); err != nil {
		return err
	}
	s.attempted, err = s.invocation.InterruptSource(ctx)
	if err != nil {
		return err
	}
	frame, err := s.acceptCompletion(challenge)
	if err != nil {
		return err
	}
	if err = s.storage.record("domain-joined", s.storage.directory, nil, digest(frame[:])); err != nil {
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
	a := started.archive
	r := sourceRetirementResult{Schema: "ardents-source-retirement-v1", Attempt: uint32(started.number), Generation: b.Generation, BindingSHA256: a.BindingSHA256, StartSHA256: started.sha256, InvocationID: a.InvocationID, PID: a.PID, Manager: a.Manager, Challenge: hex.EncodeToString(challenge[:]), CompletionSHA256: digest(frame[:]), StartedRealtime: execution.startedRealtime, StartedMonotonic: execution.startedMonotonic, ExitedRealtime: execution.exitedRealtime, ExitedMonotonic: execution.exitedMonotonic, Code: execution.code, Status: execution.status}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err = s.storage.record("native-joined", s.storage.directory, nil, digest(raw)); err != nil {
		return err
	}
	// Domain completion alone cannot release original native custody. Join,
	// then physically close the original manager/proc/cgroup before commit.
	if err = s.closeOriginal(); err != nil {
		return err
	}
	if err = s.storage.record("native-closed", s.storage.directory, nil, digest(raw)); err != nil {
		return err
	}
	if err = s.storage.writeResult(raw, s.base); err != nil {
		return err
	}
	// The independent reader consumes actual synced bytes and every phase; it
	// cannot manufacture a clean result from the writer's detached struct.
	retired, err := readSourceRetirementProof(selection, started)
	if err != nil || retired.sha256 != digest(raw) {
		return errors.Join(ErrBinding, err)
	}
	if err = s.base(); err != nil {
		return err
	}
	// Keep the blocker through all original observer/input and writer closes.
	// No later diagnostic/output can turn an uncertain prefix into retirement.
	if err = errors.Join(s.namespace.close(), s.identity.close(), s.storage.close(), s.chain.close()); err != nil {
		return err
	}
	s.namespace, s.identity, s.chain = nil, nil, nil
	return s.storage.publishRetirement()
}

func (s *sourceStop) base() error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := s.chain.initial.check(); err != nil {
		return err
	}
	b := s.chain.initial.selection.binding
	if err := observeSourcePrincipal(s.ctx, b.UID, b.GID); err != nil {
		return err
	}
	if err := s.identity.check(); err != nil {
		return err
	}
	identity, err := sourceManagerIdentity()
	if err != nil || identity != s.managerIdentity {
		return errors.Join(ErrBinding, err)
	}
	if err = s.namespace.check(); err != nil {
		return err
	}
	if s.storage != nil {
		return s.storage.check()
	}
	return s.ctx.Err()
}

func (s *sourceStop) observeOriginal() error {
	if err := s.base(); err != nil {
		return err
	}
	started := s.chain.latest
	b := s.chain.initial.selection.binding
	for range 2 {
		if err := s.invocation.Observe(s.ctx); err != nil {
			return err
		}
		if err := s.scope.Observe(); err != nil {
			return err
		}
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		if facts.Manager != started.archive.Manager {
			return ErrBinding
		}
		if err = verifySourceRunning(facts.Unit, facts.Service, b.Request, b.Generation, started.archive.PID, started.id); err != nil {
			return err
		}
		execution, err := sourceExecutionStatus(facts.Service)
		if err != nil || execution.startedRealtime != started.archive.StartedRealtime || execution.startedMonotonic != started.archive.StartedMonotonic {
			return errors.Join(ErrBinding, err)
		}
		if err = s.base(); err != nil {
			return err
		}
		if err = s.invocation.Observe(s.ctx); err != nil {
			return err
		}
		if err = s.scope.Observe(); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}

func (s *sourceStop) acceptCompletion(challenge [32]byte) (frame [sourceRetirementFrameSize]byte, err error) {
	if err = s.base(); err != nil {
		return frame, err
	}
	deadline, ok := s.ctx.Deadline()
	if !ok {
		return frame, ErrInput
	}
	listener := s.storage.listener
	if listener == nil {
		return frame, ErrBinding
	}
	if err = listener.SetDeadline(deadline); err != nil {
		return frame, err
	}
	closed := make(chan error, 1)
	stop := context.AfterFunc(s.ctx, func() { closed <- listener.Close() })
	defer func() {
		if !stop() {
			physical := <-closed
			if !errors.Is(physical, net.ErrClosed) {
				err = errors.Join(err, physical)
			}
		}
	}()
	connection, err := listener.AcceptUnix()
	if err != nil {
		return frame, err
	}
	defer func() { err = errors.Join(err, connection.Close(), s.ctx.Err()) }()
	connectionClosed := make(chan error, 1)
	stopConnection := context.AfterFunc(s.ctx, func() { connectionClosed <- connection.Close() })
	defer func() {
		if !stopConnection() {
			err = errors.Join(err, <-connectionClosed)
		}
	}()
	if err = connection.SetDeadline(deadline); err != nil {
		return frame, err
	}
	peer, err := sourceUnixPeer(connection)
	b := s.chain.initial.selection.binding
	if err != nil || peer.Pid <= 0 || uint32(peer.Pid) != s.chain.latest.archive.PID || peer.Uid != b.UID || peer.Gid != b.GID {
		return frame, errors.Join(ErrBinding, err)
	}
	if err = s.invocation.Observe(s.ctx); err != nil {
		return frame, err
	}
	if err = s.scope.Observe(); err != nil {
		return frame, err
	}
	if err = s.base(); err != nil {
		return frame, err
	}
	if count, e := io.Copy(connection, bytes.NewReader(challenge[:])); e != nil || count != int64(len(challenge)) {
		return frame, errors.Join(io.ErrShortWrite, e)
	}
	if _, err = io.ReadFull(connection, frame[:]); err != nil {
		return frame, err
	}
	start, err := completion.Encode(s.chain.latest.id, b.Generation, s.chain.latest.archive.BindingSHA256)
	if err != nil {
		return frame, err
	}
	if frame != sourceRetirementFrame(start, challenge) {
		return frame, ErrBinding
	}
	if err = s.base(); err != nil {
		return frame, err
	}
	return frame, nil
}

func (s *sourceStop) joinOriginal() {
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

func (s *sourceStop) observeCompleted() (sourceExecution, error) {
	if !s.joined {
		return sourceExecution{}, ErrBinding
	}
	b := s.chain.initial.selection.binding
	var result sourceExecution
	for range 2 {
		if err := s.base(); err != nil {
			return result, err
		}
		facts, err := s.manager.Observe()
		if err != nil {
			return result, err
		}
		if facts.Manager != s.chain.latest.archive.Manager {
			return result, ErrBinding
		}
		execution, err := sourceExecutionStatus(facts.Service)
		if err != nil {
			return result, err
		}
		if err = verifySourceCompletedStop(facts.Unit, facts.Service, b.Request, b.Generation, s.chain.latest, execution); err != nil {
			return result, err
		}
		if result != (sourceExecution{}) && result != execution {
			return result, ErrBinding
		}
		result = execution
		if err = s.base(); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *sourceStop) closeOriginal() error {
	err := errors.Join(s.storage.closeListener(), s.manager.Close(), s.scope.Close(), s.invocation.Close())
	s.manager, s.scope, s.invocation = nil, nil, nil
	return err
}

func (s *sourceStop) close() error {
	if s == nil {
		return nil
	}
	return errors.Join(s.manager.Close(), s.scope.Close(), s.invocation.Close(), s.namespace.close(), s.identity.close(), s.storage.close(), s.chain.close(), s.ctx.Err())
}
