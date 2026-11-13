package join

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/process"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// One root attempt owns all of its observations. The independently admitted
// non-root Startup borrows none of these objects or writer leases.
type startAttempt struct {
	ctx               context.Context
	verifier          *release.Verifier
	authorization     infrastructure.JoinAuthorization
	completed         Completed
	nextNumber        attemptNumber
	identity          *runtimeIdentity
	namespace         *startNamespace
	manager           *systemd.JoinControl
	managerIdentity   string
	storage           *startStorage
	barrier           *startBarrier
	invocation        process.JoinInvocation
	scope             *cgroup.JoinScope
	pid               uint32
	id                [16]byte
	job               string
	historyOpening    uint8
	predecessor       string
	intent            []byte
	attempted, handed bool
	terminal          error
}

var errControllerProgram = errors.New("join installation: original controller program differs")

// StartInitial admits only the first explicit start of a completed stopped
// Join installation. It consumes and closes the original fresh Release
// verifier before completion publication. Existing attempt storage refuses,
// including after uncertainty.
// A failed native Start retains original custody until actual kernel join;
// no deadline, process absence or stopped manager flag manufactures completion.
func StartInitial(ctx context.Context, v *release.Verifier, authorization infrastructure.JoinAuthorization, bindingPath, distribution, history string) (returned error) {
	if ctx == nil || v == nil || os.Geteuid() != 0 || !validPath(bindingPath) || !validPath(distribution) || !validPath(history) {
		return ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return ErrInput
	}
	s := &startAttempt{ctx: ctx, verifier: v, authorization: authorization}
	defer func() {
		s.terminal = errors.Join(s.terminal, returned)
		if s.attempted && !s.handed {
			s.joinFailedAttempt()
		}
		returned = errors.Join(s.terminal, s.close())
		returned = errors.Join(returned, v.Close(), ctx.Err())
		if returned == nil && s.handed {
			returned = s.storage.publish()
		}
	}()
	if err := installedPlatform(ctx); err != nil {
		return err
	}
	var err error
	s.completed, err = ReadCompleted(ctx, bindingPath)
	if err != nil {
		return err
	}
	c := s.completed.original
	if c.invocations != nil {
		return ErrBinding
	}
	if c.binding.History != history {
		return ErrBinding
	}
	if err := s.completed.IndependentPaths(distribution, history); err != nil {
		return err
	}
	if err := s.completed.CheckAuthorization(v, authorization); err != nil {
		return err
	}
	return s.launch()
}

func (s *startAttempt) launch() error {
	ctx := s.ctx
	c := s.completed.original
	bindingPath := filepath.Join(c.binding.Request.InstallationRoot, "binding.json")
	var err error
	program := c.files[filepath.Join(c.sealed, programName)]
	if err := verifyControllerProgram(ctx, program.body); err != nil {
		return errors.Join(errControllerProgram, err)
	}
	if !c.hostingPrepared {
		return ErrBinding
	}
	s.identity, err = openRootIdentity(ctx)
	if err != nil {
		return err
	}
	s.manager, err = systemd.OpenJoinControl(ctx)
	if err != nil {
		return err
	}
	s.managerIdentity, err = s.manager.ManagerIdentity()
	if err != nil {
		return err
	}
	if s.nextNumber != 0 {
		h := c.invocations
		if h == nil || len(h.starts) == 0 || h.starts[len(h.starts)-1].archive.Manager != s.managerIdentity {
			return ErrBinding
		}
	}
	facts, err := s.manager.Observe()
	if err != nil {
		return err
	}
	s.namespace, err = openStartNamespace(ctx, facts.UnitPaths)
	if err != nil {
		return err
	}
	if err := s.observeStopped(); err != nil {
		return err
	}
	s.intent, err = s.startIntentBytes()
	if err != nil {
		return err
	}
	nativeContext, err := captureStartNativeContext(s.identity, s.managerIdentity, s.namespace, c)
	if err != nil {
		return err
	}
	if err := s.observeStopped(); err != nil {
		return err
	}
	s.storage, err = createStartStorage(ctx, c.binding.Request.InstallationRoot, s.nextNumber, digest(c.files[bindingPath].body))
	c.start = s.storage
	if err != nil {
		return err
	}
	if err := s.storage.record("native-context", c.binding.Request.InstallationRoot, nil, digest(nativeContext)); err != nil {
		return err
	}
	s.barrier, err = prepareStartBarrier(s)
	if err != nil {
		return err
	}
	if err := s.observeStopped(); err != nil {
		return err
	}
	if err := s.storage.record("start-attempt", s.storage.path, nil, digest(s.barrier.guard.body)); err != nil {
		return err
	}
	if err := s.observeStopped(); err != nil {
		return err
	}
	job, err := s.manager.Start()
	s.attempted, s.job = job.Attempted, job.Object
	if err != nil {
		return err
	}
	if err := s.awaitRunning(); err != nil {
		return err
	}
	if err := s.barrier.accept(); err != nil {
		return err
	}
	if err := s.archive(); err != nil {
		return err
	}
	if err := s.barrier.acknowledge(); err != nil {
		return err
	}
	if err := s.checkCompletedTranscript(); err != nil {
		return err
	}
	return s.observeRunning()
}

func verifyControllerProgram(ctx context.Context, wanted []byte) (returned error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.Open("/proc/self/exe") // intentional original mapped executable
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, file.Close(), ctx.Err()) }()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() != int64(len(wanted)) {
		return errors.Join(ErrBinding, err)
	}
	raw, err := io.ReadAll(io.LimitReader(file, int64(len(wanted))+1))
	if err != nil || !bytes.Equal(raw, wanted) {
		return errors.Join(ErrBinding, err)
	}
	after, err := file.Stat()
	if err != nil || !sameInputFile(before, after) {
		return errors.Join(ErrBinding, err)
	}
	return ctx.Err()
}

func (s *startAttempt) base() error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := s.completed.CheckAuthorization(s.verifier, s.authorization); err != nil {
		return err
	}
	if err := s.identity.check(); err != nil {
		return err
	}
	identity, err := s.manager.ManagerIdentity()
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
	return s.ctx.Err()
}

func (s *startAttempt) bindingBytes() []byte {
	c := s.completed.original
	return c.files[filepath.Join(c.binding.Request.InstallationRoot, "binding.json")].body
}

func (s *startAttempt) checkCompletedTranscript() error {
	frame, err := s.barrier.frame()
	if err != nil {
		return err
	}
	storage, barrier := s.storage, s.barrier
	ack, err := barrier.ackFrame()
	if err != nil {
		return err
	}
	recorded, err := decodeStartRecordsFrames(storage.journal.body, storage.number, storage.journal.path, storage.path, digest(s.bindingBytes()), digest(barrier.guard.body), digest(barrier.record.body), digest(barrier.archive.body), digest(frame[:]), digest(ack[:]), s.completed.original.binding.GID)
	if err != nil {
		return err
	}
	for _, original := range append([]*initialFile{storage.journal}, storage.files...) {
		native, ok := infoSys(original.info)
		r, exists := recorded[original.path]
		if !ok || !exists || r.Device != uint64(native.Dev) || r.Inode != native.Ino || r.Mode != uint32(original.info.Mode().Perm()) || r.UID != native.Uid || r.GID != native.Gid {
			return ErrBinding
		}
	}
	for path, info := range map[string]os.FileInfo{storage.path: storage.dirInfo, filepath.Join(storage.path, "start-completion.socket"): barrier.socket} {
		native, ok := info.Sys().(*syscall.Stat_t)
		r, exists := recorded[path]
		if !ok || !exists || r.Device != uint64(native.Dev) || r.Inode != native.Ino || r.Mode != uint32(info.Mode().Perm()) || r.UID != native.Uid || r.GID != native.Gid {
			return ErrBinding
		}
	}
	return s.observeRunning()
}

func (s *startAttempt) observeStopped() error {
	if err := s.base(); err != nil {
		return err
	}
	b := s.completed.original.binding
	for round := 0; round < 2; round++ {
		if err := cgroup.CheckJoinQuiescent(s.ctx); err != nil {
			return err
		}
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		checkStopped := verifyJoinStopped(facts.Unit, facts.Service, b.Request, b.Generation)
		if s.nextNumber != 0 {
			h := s.completed.original.invocations
			if h == nil || len(h.retirements) != len(h.starts) {
				return ErrBinding
			}
			old := h.starts[len(h.starts)-1]
			ret := h.retirements[len(h.retirements)-1]
			checkStopped = verifyJoinRestartStopped(facts.Unit, facts.Service, b.Request, b.Generation, old.archive, old.id, ret.result.execution())
			if old.binding.Generation != b.Generation {
				checkStopped = verifyJoinReplacementStopped(facts.Unit, facts.Service, b, old, ret)
			}
		}
		if err := checkStopped; err != nil {
			return err
		}
		if err := s.base(); err != nil {
			return err
		}
		if err := cgroup.CheckJoinQuiescent(s.ctx); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}

func (s *startAttempt) awaitRunning() error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
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
			s.pid, s.id, err = runningInvocation(facts.Unit, facts.Service)
			if err != nil {
				return err
			}
			b := s.completed.original.binding
			program := s.completed.original.files[filepath.Join(s.completed.original.sealed, programName)]
			s.scope, err = cgroup.RetainJoinStarted(s.ctx, s.pid)
			if err != nil {
				return err
			}
			s.invocation, err = process.RetainJoin(s.ctx, process.JoinExpected{PID: s.pid, UID: b.UID, GID: b.GID, Invocation: s.id, InstallationRoot: b.Request.InstallationRoot, Generation: b.Generation, ProgramIdentity: program.info})
			if err != nil {
				return err
			}
			if err := s.observeRunning(); err != nil {
				return err
			}
			return s.storage.record("invocation-observed", s.storage.path, nil, digest(s.barrier.guard.body))
		}
		pending := systemd.Matches(facts.Unit, "ActiveState", "s", "activating") ||
			(systemd.Matches(facts.Unit, "ActiveState", "s", "inactive") && systemd.Matches(facts.Unit, "SubState", "s", "dead")) || (active && running)
		if !pending || !ownQueuedJob(facts.Unit, s.job) {
			return ErrBinding
		}
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-ticker.C:
		}
	}
}

func ownQueuedJob(unit systemd.Properties, object string) bool {
	const prefix = "/org/freedesktop/systemd1/job/"
	if !strings.HasPrefix(object, prefix) {
		return false
	}
	text := strings.TrimPrefix(object, prefix)
	id, err := strconv.ParseUint(text, 10, 32)
	return err == nil && id != 0 && strconv.FormatUint(id, 10) == text && systemd.Matches(unit, "Job", "(uo)", []any{uint32(id), object})
}

func runningInvocation(unit, service systemd.Properties) (pid uint32, id [16]byte, err error) {
	p, ok := service["MainPID"]
	i, yes := unit["InvocationID"]
	var parts []json.RawMessage
	if !ok || !yes || p.Type != "u" || i.Type != "ay" || systemd.Decode(p.Data, &pid) != nil || pid == 0 || systemd.Decode(i.Data, &parts) != nil || len(parts) != len(id) {
		return 0, [16]byte{}, ErrBinding
	}
	for index, part := range parts {
		if systemd.Decode(part, &id[index]) != nil {
			return 0, [16]byte{}, ErrBinding
		}
	}
	if id == [16]byte{} {
		return 0, [16]byte{}, ErrBinding
	}
	return pid, id, nil
}

func (s *startAttempt) observeRunning() error {
	if s.pid == 0 || s.id == [16]byte{} || s.scope == nil {
		return ErrBinding
	}
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
	b := s.completed.original.binding
	for round := 0; round < 2; round++ {
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		if err := verifyJoinRunning(facts.Unit, facts.Service, b.Request, b.Generation, s.pid, s.id); err != nil {
			return err
		}
		if err := base(); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}

func (s *startAttempt) joinFailedAttempt() {
	// Unknown original scope is deliberately not replaced with a newly observed
	// empty path. Keep all custody and the failed result; recovery is separate.
	if s.scope == nil {
		select {}
	}
	joined, err := s.scope.Joined()
	s.terminal = errors.Join(s.terminal, err)
	if !joined {
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
	retainedJoinFailure := false
	for !joined {
		<-ticker.C
		joined, err = s.scope.Joined()
		if err != nil && !retainedJoinFailure {
			s.terminal = errors.Join(s.terminal, err)
			retainedJoinFailure = true
		}
	}
}

func (s *startAttempt) close() error {
	if s == nil {
		return nil
	}
	// Initial's independent exclusive lease returns last, after the new attempt
	// and every original native observer. No runtime borrower owns these objects.
	physical := errors.Join(s.barrier.close(), s.manager.Close(), s.namespace.close(), s.identity.close(), s.scope.Close(), s.invocation.Close(), s.storage.close())
	physical = errors.Join(physical, s.completed.Close())
	clear(s.intent)
	return errors.Join(physical, s.ctx.Err())
}
