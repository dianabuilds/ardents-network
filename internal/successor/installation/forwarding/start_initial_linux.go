package forwarding

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
	authorization     infrastructure.ForwardingAuthorization
	completed         Completed
	history           Started
	nextNumber        attemptNumber
	identity          *runtimeIdentity
	namespace         *startNamespace
	manager           *systemd.ForwardingControl
	managerIdentity   string
	storage           *startStorage
	barrier           *startBarrier
	invocation        process.ForwardingInvocation
	scope             *cgroup.ForwardingScope
	pid               uint32
	id                [16]byte
	job               string
	predecessor       string
	historyOpening    uint8
	intent            []byte
	attempted, handed bool
	terminal          error
}

var errControllerProgram = errors.New("forwarding installation: original controller program differs")

// StartInitial admits only the first explicit start of a completed stopped
// Forwarding installation. It consumes and closes the original fresh Release
// verifier before completion publication. Existing attempt storage refuses,
// including after uncertainty.
// A failed native Start retains original custody until actual kernel join;
// no deadline, process absence or stopped manager flag manufactures completion.
func StartInitial(ctx context.Context, v *release.Verifier, authorization infrastructure.ForwardingAuthorization, bindingPath, distribution, history string) (returned error) {
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
		returned = errors.Join(returned, v.Close(), finishTransition(ctx))
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
	if c.binding.History != history {
		return ErrBinding
	}
	if err := s.completed.IndependentPaths(distribution, history); err != nil {
		return err
	}
	if err := s.completed.CheckAuthorization(v, authorization); err != nil {
		return err
	}
	if chain := c.files.invocations; chain != nil && chain.head() != nil {
		if chain.snapshot != nil || len(chain.starts) != 0 || chain.head().number+1 >= attemptLimit {
			return ErrBinding
		}
		predecessor, err := chain.head().predecessor()
		if err != nil {
			return err
		}
		s.history = Started{original: &started{invocationChain: chain, ctx: ctx, verifier: v, completed: s.completed}}
		s.nextNumber = chain.head().number + 1
		s.predecessor = predecessor
	}
	return s.launch()
}

func (s *startAttempt) launch() error {
	ctx := s.ctx
	c := s.completed.original
	bindingPath := filepath.Join(c.binding.Request.InstallationRoot, "binding.json")
	var err error
	program := c.files.files[filepath.Join(c.files.sealed, programName)]
	if err := verifyControllerProgram(ctx, program.body); err != nil {
		return errors.Join(errControllerProgram, err)
	}
	if err := c.readHostingPreparation(bindingPath); err != nil {
		return err
	}
	s.identity, err = openRootIdentity(ctx)
	if err != nil {
		return err
	}
	s.manager, err = systemd.OpenForwardingControl(ctx)
	if err != nil {
		return err
	}
	s.managerIdentity, err = s.manager.ManagerIdentity()
	if err != nil {
		return err
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
	// This choice comes from independently completed original runtime history,
	// never an attempt number, a plan flag or a missing semantic-root pathname.
	// Completed never-started entries retain their own proof but do not create
	// runtime history. Every actual runtime must still have a genuine archive.
	if h := s.history.original; h != nil {
		if err := s.history.Check(); err != nil {
			return err
		}
		for _, prior := range h.starts {
			if prior == nil || prior.archive.PID == 0 {
				return ErrBinding
			}
			s.historyOpening = 1
		}
	}
	s.intent, err = s.startIntentBytes()
	if err != nil {
		return err
	}
	nativeContext, err := captureNativeContext(s.identity, s.managerIdentity, s.namespace, nil, c.files)
	if err != nil {
		return err
	}
	if err := s.observeStopped(); err != nil {
		return err
	}
	if err := bindStartTransition(ctx, s.nextNumber.journal(c.binding.Request.InstallationRoot), c.binding.Generation, c.files.files[bindingPath].body, nativeContext, s.managerIdentity, s.intent); err != nil {
		return err
	}
	s.storage, err = createStartStorage(ctx, c.binding.Request.InstallationRoot, s.nextNumber, digest(c.files.files[bindingPath].body))
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
	if s.history.original != nil {
		if err := s.history.checkWithEffects(s.storage, nil); err != nil {
			return err
		}
	} else if err := s.firstAttemptNamespace(); err != nil {
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

func (s *startAttempt) firstAttemptNamespace() error {
	r := s.completed.original.binding.Request
	directory := s.completed.original.files.dirs[r.InstallationRoot]
	if directory == nil {
		return ErrBinding
	}
	if _, err := directory.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	entries, err := directory.file.ReadDir(4)
	if err != nil && err != io.EOF || len(entries) < 2 || len(entries) > 3 {
		return errors.Join(ErrBinding, err)
	}
	for _, entry := range entries {
		switch entry.Name() {
		case "binding.json":
			if !entry.Type().IsRegular() {
				return ErrBinding
			}
		case "generations":
			if !entry.IsDir() {
				return ErrBinding
			}
		case "forwarding-startup":
			if s.storage == nil || s.storage.dir == nil || !entry.IsDir() {
				return ErrBinding
			}
		default:
			return ErrBinding
		}
	}
	for n := attemptNumber(0); n < attemptLimit; n++ {
		for _, path := range []string{n.journal(r.InstallationRoot), n.directory(r.InstallationRoot), n.retirement(r.InstallationRoot), n.retirementJournal(r.InstallationRoot)} {
			if n == 0 && s.storage != nil && (path == n.journal(r.InstallationRoot) || path == s.storage.path) {
				// Its own partial journal/directory has separate exact inode/byte
				// custody. No other retained attempt may enter a first start.
				continue
			}
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				return errors.Join(ErrBinding, err)
			}
		}
	}
	return s.ctx.Err()
}

func (s *startAttempt) bindingBytes() []byte {
	c := s.completed.original
	return c.files.files[filepath.Join(c.binding.Request.InstallationRoot, "binding.json")].body
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
		if err := cgroup.CheckForwardingQuiescent(s.ctx); err != nil {
			return err
		}
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		if s.history.original == nil || s.history.original.snapshot == nil {
			if err := verifyForwardingStopped(facts.Unit, facts.Service, b.Request, b.Generation); err != nil {
				return err
			}
		} else {
			h := s.history.original
			if h.latestRetirement == nil || s.managerIdentity != h.snapshot.archive.Manager {
				return ErrBinding
			}
			selected, ok := h.completed.original.selections[digest(s.bindingBytes())]
			if !ok {
				return ErrBinding
			}
			var policy error
			if b.Generation == h.snapshot.archive.Generation {
				policy = verifyForwardingRestartStopped(facts.Unit, facts.Service, b.Request, b.Generation, h.snapshot.archive, h.snapshot.id, h.latestRetirement.result.execution())
			} else {
				policy = verifyForwardingReplacementStopped(facts.Unit, facts.Service, selected, h.snapshot.archive, h.snapshot.number, h.snapshot.id, h.latestRetirement.result, h.latestRetirement.sha256)
			}
			if err := policy; err != nil {
				return err
			}
		}
		if err := s.base(); err != nil {
			return err
		}
		if err := cgroup.CheckForwardingQuiescent(s.ctx); err != nil {
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
			program := s.completed.original.files.files[filepath.Join(s.completed.original.files.sealed, programName)]
			s.scope, err = cgroup.RetainForwardingStarted(s.ctx, s.pid)
			if err != nil {
				return err
			}
			s.invocation, err = process.RetainForwarding(s.ctx, process.ForwardingExpected{PID: s.pid, UID: b.UID, GID: b.GID, Invocation: s.id, InstallationRoot: b.Request.InstallationRoot, Generation: b.Generation, ProgramIdentity: program.info})
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
	if s.history.original != nil {
		for _, old := range s.history.original.starts {
			if old.id == s.id {
				return ErrBinding
			}
		}
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
		if err := verifyForwardingRunning(facts.Unit, facts.Service, b.Request, b.Generation, s.pid, s.id); err != nil {
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
	if s.history.original != nil {
		physical = errors.Join(physical, s.history.Close())
	} else {
		physical = errors.Join(physical, s.completed.Close())
	}
	clear(s.intent)
	return errors.Join(physical, s.ctx.Err())
}
