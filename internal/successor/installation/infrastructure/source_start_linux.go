package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/process"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// One original start attempt owns its distinct writer lease and native
// observers. The runtime independently opens every observation; it borrows none
// of these objects. An irreversible full ACK remains a handoff despite a later
// journal, cancellation or physical Close failure.
type sourceStartAttempt struct {
	ctx               context.Context
	verifier          *release.Verifier
	authorization     Authorization
	initial           *sourceInitial
	identity          *sourceRuntimeIdentity
	managerIdentity   string
	namespace         *sourceNamespace
	manager           *systemd.SourceControl
	storage           *sourceStartStorage
	barrier           *sourceStartBarrier
	invocation        *process.Invocation
	scope             *cgroup.SourceScope
	pid               uint32
	job               string
	id                [16]byte
	attempted, handed bool
	terminal          error
	predecessor       *sourceRetirement
}

// StartSource admits only the first explicit start of an already selected
// stopped Source. The caller retains its original Release verifier through this
// operation's return. Existing first-start storage always refuses; no restart,
// continuation, adoption, rollback or receipt-as-permission path exists.
// On uncertain Start, a return requires original kernel scope join. Missing
// original join can remain unavailable indefinitely; the command reports that
// bounded notice while retaining this original operation and all its custody.
func StartSource(ctx context.Context, v *release.Verifier, a Authorization, binding, history string) (err error) {
	return startSourceAttempt(ctx, v, a, binding, history, nil)
}

// RestartSource admits a new exclusive invocation only after independent full
// original retirement readback under the original first journal's lease. It
// repeats fresh artifact/Release/native admission and never reopens or removes
// the first-start barrier as a new operation.
func RestartSource(ctx context.Context, v *release.Verifier, a Authorization, binding, history string) (err error) {
	retired, err := openSourceRetirement(ctx, v, binding, history)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, retired.close()) }()
	if retired.chain.latest.number+1 >= sourceAttemptLimit {
		return ErrBinding
	}
	return startSourceAttempt(ctx, v, a, binding, history, retired)
}

func startSourceAttempt(ctx context.Context, v *release.Verifier, a Authorization, binding, history string, predecessor *sourceRetirement) (err error) {
	if ctx == nil || v == nil || os.Geteuid() != 0 {
		return ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return ErrInput
	}
	s := &sourceStartAttempt{ctx: ctx, verifier: v, authorization: a, predecessor: predecessor}
	defer func() {
		s.terminal = errors.Join(s.terminal, err)
		if s.attempted && !s.handed {
			s.joinFailedAttempt()
		}
		err = errors.Join(s.terminal, s.close())
		if err == nil && s.handed {
			err = s.storage.publishStart()
		}
	}()
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = sourceInstalledPlatform(ctx); err != nil {
		return err
	}
	s.initial, err = openSourceInitial(ctx, v, binding, history)
	if err != nil {
		return err
	}
	selection := s.initial.selection
	b := selection.binding
	// A complete final record is not proof that the original creator returned.
	// This independent shared lock excludes its original exclusive writer until
	// it has joined every observer/helper and released the initial descriptor.
	originalJournal := selection.files[b.Request.InstallationRoot+".source-initial-journal"]
	if originalJournal == nil {
		return ErrBinding
	}
	if err = syscall.Flock(int(originalJournal.file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		return err
	}
	if err = s.initial.check(); err != nil {
		return err
	}
	if err = a.Check(ctx, v); err != nil {
		return err
	}
	get := func(name string) []byte { return selection.files[filepath.Join(selection.sealed, name)].raw }
	if !bytes.Equal(a.Descriptor(), get(DescriptorName)) || !bytes.Equal(a.Program(), get(ProgramName)) || !bytes.Equal(a.Template(), get(TemplateName)) {
		return ErrBinding
	}
	if err = verifySourceControllerProgram(ctx, get(ProgramName)); err != nil {
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
	if err = s.observeStopped(); err != nil {
		return err
	}
	if predecessor == nil {
		s.storage, err = openSourceStartStorage(ctx, selection)
	} else {
		if err = predecessor.check(); err != nil {
			return err
		}
		s.storage, err = openSourceInvocationStorage(ctx, selection, predecessor.chain.latest.number+1)
	}
	if err != nil {
		return err
	}
	s.barrier, err = prepareSourceStartBarrier(s)
	if err != nil {
		return err
	}
	if err = s.observeStopped(); err != nil {
		return err
	}
	if err = s.storage.record("start-attempt", s.storage.path, nil, digest(s.barrier.guard.body)); err != nil {
		return err
	}
	if err = s.observeStopped(); err != nil {
		return err
	}
	job, err := s.manager.Start()
	s.attempted = job.Attempted
	s.job = job.Object
	if err != nil {
		return err
	}
	if err = s.awaitRunning(); err != nil {
		return err
	}
	if err = s.barrier.accept(); err != nil {
		return err
	}
	if err = s.archive(); err != nil {
		return err
	}
	if err = s.barrier.acknowledge(); err != nil {
		return err
	}
	return s.checkCompletedTranscript()
}

// This writer-side check exercises the same complete grammar that a later
// independent stop reader must consume. It neither proves domain retirement
// nor releases the first-start barrier for another attempt.
func (s *sourceStartAttempt) checkCompletedTranscript() error {
	frame, err := s.barrier.frame()
	if err != nil {
		return err
	}
	storage, barrier := s.storage, s.barrier
	binding := s.initial.selection.files[filepath.Join(s.initial.selection.binding.Request.InstallationRoot, "binding.json")].raw
	recorded, err := decodeSourceStartRecords(storage.journal.body, storage.number, storage.journal.path, storage.path,
		digest(binding), digest(barrier.guard.body), digest(barrier.record.body), digest(barrier.archive.body), digest(frame[:]), s.initial.selection.binding.GID)
	if err != nil {
		return err
	}
	for _, original := range append([]*sourceBirth{storage.journal}, storage.files...) {
		native, ok := original.info.Sys().(*syscall.Stat_t)
		r, exists := recorded[original.path]
		if !ok || !exists || r.Device != uint64(native.Dev) || r.Inode != native.Ino || r.Mode != uint32(original.info.Mode().Perm()) || r.UID != native.Uid || r.GID != native.Gid {
			return ErrBinding
		}
	}
	return s.observeRunning()
}

func verifySourceControllerProgram(ctx context.Context, wanted []byte) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	file, err := os.Open("/proc/self/exe") // intentional original mapped executable
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close(), ctx.Err()) }()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() != int64(len(wanted)) {
		return errors.Join(ErrBinding, err)
	}
	raw, err := io.ReadAll(io.LimitReader(file, int64(len(wanted))+1))
	if err != nil || !bytes.Equal(raw, wanted) {
		return errors.Join(ErrBinding, err)
	}
	after, err := file.Stat()
	if err != nil || !sameSourceFile(before, after) {
		return errors.Join(ErrBinding, err)
	}
	return ctx.Err()
}

func (s *sourceStartAttempt) base() error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := s.initial.check(); err != nil {
		return err
	}
	if err := s.authorization.Check(s.ctx, s.verifier); err != nil {
		return err
	}
	b := s.initial.selection.binding
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
	if err := s.namespace.check(); err != nil {
		return err
	}
	if s.storage != nil {
		if err := s.storage.check(); err != nil {
			return err
		}
	}
	if s.barrier != nil {
		if err := s.barrier.observe(); err != nil {
			return err
		}
	}
	if s.predecessor != nil {
		if err := s.predecessor.check(); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}

func (s *sourceStartAttempt) observeStopped() error {
	b := s.initial.selection.binding
	if err := s.base(); err != nil {
		return err
	}
	for round := 0; round < 2; round++ {
		if err := cgroup.CheckSourceQuiescent(s.ctx); err != nil {
			return err
		}
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		if s.predecessor == nil {
			err = verifySourceStopped(facts.Unit, facts.Service, b.Request, b.Generation)
		} else {
			err = s.predecessor.stopped(facts)
		}
		if err != nil {
			return err
		}
		if err = s.base(); err != nil {
			return err
		}
		if err = cgroup.CheckSourceQuiescent(s.ctx); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}

func (s *sourceStartAttempt) awaitRunning() error {
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	for {
		if err := s.base(); err != nil {
			return err
		}
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		active := systemd.Matches(facts.Unit, "ActiveState", "s", "active")
		running := systemd.Matches(facts.Unit, "SubState", "s", "running")
		noJob := systemd.Matches(facts.Unit, "Job", "(uo)", []any{uint32(0), "/"})
		if active && running && noJob {
			s.pid, s.id, err = sourceInvocation(facts.Unit, facts.Service)
			if err != nil {
				return err
			}
			if s.predecessor != nil && s.id == s.predecessor.chain.latest.id {
				return ErrBinding
			}
			b := s.initial.selection.binding
			program := s.initial.selection.files[filepath.Join(s.initial.selection.sealed, ProgramName)]
			s.scope, err = cgroup.RetainSourceStarted(s.ctx, s.pid)
			if err != nil {
				return err
			}
			s.invocation, err = process.RetainSource(s.ctx, process.SourceExpected{PID: s.pid, UID: b.UID, GID: b.GID, Invocation: s.id, InstallationRoot: b.Request.InstallationRoot, Generation: b.Generation, ProgramIdentity: program.info})
			if err != nil {
				return err
			}
			if err = s.observeRunning(); err != nil {
				return err
			}
			return s.storage.record("invocation-observed", s.storage.path, nil, digest(s.barrier.guard.body))
		}
		// Waiting for this one asynchronous job grants no runtime admission. Any
		// terminal/unknown native state refuses; there is no second Start or retry.
		pending := systemd.Matches(facts.Unit, "ActiveState", "s", "activating") ||
			(systemd.Matches(facts.Unit, "ActiveState", "s", "inactive") && systemd.Matches(facts.Unit, "SubState", "s", "dead")) || (active && running)
		if !pending || !sourceOwnQueuedJob(facts.Unit, s.job) {
			return ErrBinding
		}
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-timer.C:
		}
	}
}

func sourceOwnQueuedJob(unit systemd.Properties, object string) bool {
	const prefix = "/org/freedesktop/systemd1/job/"
	if !strings.HasPrefix(object, prefix) {
		return false
	}
	text := strings.TrimPrefix(object, prefix)
	id, err := strconv.ParseUint(text, 10, 32)
	return err == nil && id != 0 && strconv.FormatUint(id, 10) == text &&
		systemd.Matches(unit, "Job", "(uo)", []any{uint32(id), object})
}

func (s *sourceStartAttempt) observeRunning() error {
	if s.pid == 0 || s.id == [16]byte{} || s.invocation == nil || s.scope == nil {
		return ErrBinding
	}
	b := s.initial.selection.binding
	base := func() error {
		if err := s.base(); err != nil {
			return err
		}
		if err := s.invocation.Observe(s.ctx); err != nil {
			return err
		}
		return s.scope.Observe()
	}
	if err := base(); err != nil {
		return err
	}
	for round := 0; round < 2; round++ {
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		if err = verifySourceRunning(facts.Unit, facts.Service, b.Request, b.Generation, s.pid, s.id); err != nil {
			return err
		}
		if err = base(); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}

func (s *sourceStartAttempt) joinFailedAttempt() {
	// No timeout, PID absence, manager inactive flag or replacement scope can
	// manufacture original completion. Keep unknown custody even after failure.
	if s.scope == nil {
		select {}
	}
	joined, err := s.scope.Joined()
	s.terminal = errors.Join(s.terminal, err)
	if !joined && s.invocation != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), 3*time.Second)
		physical := s.invocation.Observe(cleanup)
		if physical == nil {
			_, physical = s.manager.Stop(cleanup)
		}
		s.terminal = errors.Join(s.terminal, physical)
		cancel()
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	firstJoinError := false
	for !joined {
		<-ticker.C
		joined, err = s.scope.Joined()
		if err != nil && !firstJoinError {
			s.terminal = errors.Join(s.terminal, err)
			firstJoinError = true
		}
	}
}

func (s *sourceStartAttempt) close() error {
	if s == nil {
		return nil
	}
	// Failed attempts have joined the original scope above. Full ACK transfers
	// startup to the independently observing runtime; none of these root objects
	// is borrowed by it. Its own State/process completion remains separately owned.
	return errors.Join(s.barrier.close(), s.manager.Close(), s.namespace.close(), s.identity.close(), s.scope.Close(), s.invocation.Close(), s.initial.close(), s.predecessor.close(), s.storage.close(), s.ctx.Err())
}
