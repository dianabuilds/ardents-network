package launch

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/generation"
	"github.com/dianabuilds/ardents-network/internal/platform/process"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// Startup retains independently observed installed bytes, the original service
// process and platform manager. It conveys no Release, Job or
// Service authority. Composition must obtain those rights from their owners.
type Startup struct {
	lifetime *startupLifetime
}

type startupLifetime struct {
	mu         sync.Mutex
	inspection *startupInspection
	closed     bool
	result     error
}

// OpenStartup observes this actual non-root invocation and its original
// artifact, account and platform manager before yielding plans.
func OpenStartup(ctx context.Context, directory string) (*Startup, error) {
	inspection, err := openStartupInspection(ctx, directory)
	if err != nil {
		return nil, err
	}
	return &Startup{lifetime: &startupLifetime{inspection: inspection}}, nil
}

// Check reobserves the same retained installed invocation at an INNER effect
// bracket using identity-only granularity: sealed generation and retained file
// identity (dev/ino/mode/size/ModTime/uid/gid/nlink/Ctim) plus the original
// process, manager and account observations, without re-reading file content.
// Full content is reserved for operation/admission boundaries (CheckFull). An
// in-place root rewrite with forged timestamps between boundaries is detected at
// the next CheckFull; that root-tamper latency bound (the operation cadence) is
// the admitted trade of the identity-only inner granularity.
var qualificationStartupChecks atomic.Uint64
var qualificationStartupWait atomic.Int64
var qualificationStartupWork atomic.Int64

func (s *Startup) Check() error {
	start := time.Now()
	if s == nil || s.lifetime == nil {
		return ErrInput
	}
	s.lifetime.mu.Lock()
	acquired := time.Now()
	err := s.lifetime.checkLocked(s.lifetime.inspection.observeIdentity)
	s.lifetime.mu.Unlock()
	wait := qualificationStartupWait.Add(int64(acquired.Sub(start)))
	work := qualificationStartupWork.Add(int64(time.Since(acquired)))
	count := qualificationStartupChecks.Add(1)
	if count%128 == 0 {
		fmt.Fprintf(os.Stderr, "qualification startup checks=%d mean-wait=%s mean-observe=%s\n", count, time.Duration(wait/int64(count)), time.Duration(work/int64(count)))
	}
	return err
}

// CheckFull reobserves the retained installed invocation with FULL content at an
// operation/admission boundary: every sealed generation and retained file is
// freshly re-read and byte-compared, detecting an in-place content rewrite even
// with forged ModTime/Ctim. Operation entry points and admission handoffs use
// this; inner effect brackets use Check.
func (s *Startup) CheckFull() error {
	if s == nil || s.lifetime == nil {
		return ErrInput
	}
	s.lifetime.mu.Lock()
	defer s.lifetime.mu.Unlock()
	return s.lifetime.checkLocked(s.lifetime.inspection.observe)
}

func (s *startupLifetime) checkLocked(observe func() error) error {
	if s.closed || s.inspection == nil {
		return errors.Join(ErrBinding, s.result)
	}
	if s.result != nil {
		return s.result
	}
	s.result = observe()
	return s.result
}

// Plans yields detached bound declarations only after original observation.
// Copied declarations neither retain this lifetime nor authorize their consumer.
func (s *Startup) Plans() ([]byte, []byte, error) {
	if s == nil || s.lifetime == nil {
		return nil, nil, ErrInput
	}
	s.lifetime.mu.Lock()
	defer s.lifetime.mu.Unlock()
	if err := s.lifetime.checkLocked(s.lifetime.inspection.observe); err != nil {
		return nil, nil, err
	}
	headless := bytes.Clone(s.lifetime.inspection.checked.files["headless.json"])
	source := bytes.Clone(s.lifetime.inspection.checked.files["source.json"])
	if err := s.lifetime.checkLocked(s.lifetime.inspection.observe); err != nil {
		return nil, nil, err
	}
	return headless, source, nil
}

// Close joins original read custody once and retains its first terminal result.
func (s *Startup) Close() error {
	if s == nil || s.lifetime == nil {
		return nil
	}
	s.lifetime.mu.Lock()
	defer s.lifetime.mu.Unlock()
	if !s.lifetime.closed {
		s.lifetime.closed = true
		s.lifetime.result = errors.Join(s.lifetime.result, s.lifetime.inspection.close())
		s.lifetime.inspection = nil
	}
	return s.lifetime.result
}

// startupInspection owns independent service-account observations. It retains
// the original caller, process and sealed snapshot until physical close.
type startupInspection struct {
	ctx        context.Context
	root       *os.Root
	identity   os.FileInfo
	files      *installedFiles
	checked    inspectedGeneration
	snapshot   *generation.Snapshot
	invocation *process.Invocation
	id         [16]byte
	manager    string // original unique bus owner, never rebound

	terminal error
}

func openStartupInspection(ctx context.Context, directory string) (result *startupInspection, returnedErr error) {
	if err := observeStartupPlatform(ctx); err != nil {
		return nil, err
	}
	if !canonicalPath(directory) || directory == "/" {
		return nil, ErrInput
	}
	ancestors, err := rootDirectoryAncestors(filepath.Dir(directory))
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(directory)
	if err != nil || info == nil {
		return nil, errors.Join(ErrBinding, err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid == 0 || native.Gid != uint32(os.Getegid()) || info.Mode() != os.ModeDir|0750 {
		return nil, ErrBinding
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	ancestors[directory] = info
	s := &startupInspection{ctx: ctx, root: root, identity: info, files: &installedFiles{
		directory: directory, gid: native.Gid, files: make(map[string]fileObservation),
		directories: ancestors, mutableDirectories: make(map[string]os.FileInfo),
	}}
	defer func() {
		if returnedErr != nil {
			s.terminal = returnedErr
			returnedErr = s.close()
			result = nil
		}
	}()
	if err := refusePlatformTransitionRecords(directory); err != nil {
		return nil, err
	}
	selectedRaw, err := s.files.read(ctx, filepath.Join(directory, "selection.json"), 4<<10, 0640, native.Gid)
	if err != nil {
		return nil, err
	}
	var selected generationSelection
	if decodeCanonical(selectedRaw, 4<<10, &selected) != nil || !canonicaldigest.Valid(selected.GenerationDigest) {
		return nil, ErrBinding
	}
	parent := filepath.Join(directory, "generations")
	sealed := filepath.Join(parent, selected.GenerationDigest)
	for _, name := range []string{parent, sealed} {
		if err := s.files.pinGenerationDirectory(name); err != nil {
			return nil, err
		}
	}
	binding, files, err := s.files.readSealedGeneration(ctx, selected.GenerationDigest)
	if err != nil {
		return nil, err
	}
	s.checked, err = inspectGeneration(directory, selectedRaw, binding, files)
	if err != nil || s.checked.binding.UID != uint32(os.Geteuid()) || s.checked.binding.GID != native.Gid {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := s.files.observeAccountAndRoots(s.checked); err != nil {
		return nil, err
	}
	if err := s.files.inspectFixedResources(ctx, s.checked); err != nil {
		return nil, err
	}
	// This second opening is retained by startup itself, independently of the
	// short-lived byte read. Matching every original file rejects substitution
	// between those openings; it never renews the initial observations.
	s.snapshot, err = generation.OpenSnapshot(ctx, parent, s.files.directories[parent], s.files.directories[sealed], selected.GenerationDigest, native.Gid)
	if err != nil {
		return nil, err
	}
	for _, name := range generation.Names() {
		if !s.snapshot.Matches(name, s.files.files[filepath.Join(sealed, name)].identity) {
			return nil, ErrBinding
		}
	}
	invocation, err := hex.DecodeString(os.Getenv("INVOCATION_ID"))
	if err != nil || len(invocation) != len(s.id) || hex.EncodeToString(invocation) != os.Getenv("INVOCATION_ID") {
		return nil, ErrBinding
	}
	copy(s.id[:], invocation)
	s.invocation, err = process.RetainSelf(ctx, process.Expected{
		PID: uint32(os.Getpid()), UID: s.checked.binding.UID, GID: s.checked.binding.GID,
		Invocation: s.id, InstallationRoot: directory, Generation: selected.GenerationDigest,
		ProgramIdentity: s.files.files[filepath.Join(sealed, "ardents-linux-amd64")].identity,
	})
	if err != nil {
		return nil, err
	}
	// The retained Snapshot now owns every sealed file after matching its
	// original identity and retaining the invocation's executable identity.
	// It fully reobserves them before and after the manager, independently of
	// root inspection. Keep selection, fixed resources and all ancestors in
	// this startup's other reader, without rereading the same sealed files.
	for _, name := range generation.Names() {
		delete(s.files.files, filepath.Join(sealed, name))
	}
	if err := s.observe(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *startupInspection) observe() error { return s.observeGranular(true) }

// observeIdentity is the inner-bracket observation: file identity only, no
// content re-read. The process, manager and account observations are retained
// (they verify identity/properties, not sealed file content).
func (s *startupInspection) observeIdentity() error { return s.observeGranular(false) }

// observeGranular keeps one before/after bracket order for both granularities.
// full selects fresh full-content re-reads of the sealed generation and retained
// files (operation/admission boundaries); otherwise identity-only re-verification
// (inner brackets). The root identity, account/roots, invocation and manager
// observations are identical in both, and failure is terminal in both.
func (s *startupInspection) observeGranular(full bool) (returnedErr error) {
	if s == nil || s.ctx == nil || s.root == nil || s.files == nil || s.snapshot == nil || s.invocation == nil {
		return ErrInput
	}
	if s.terminal != nil {
		return s.terminal
	}
	defer func() {
		if returnedErr != nil {
			s.terminal = returnedErr
		}
	}()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	current, err := s.root.Stat(".")
	if err != nil || !sameObservedDirectory(s.identity, current) {
		return errors.Join(ErrBinding, err)
	}
	observeFiles, observeSnapshot := s.files.observeIdentity, s.snapshot.ObserveIdentity
	if full {
		observeFiles, observeSnapshot = s.files.observe, s.snapshot.Observe
	}
	if err := observeFiles(s.ctx); err != nil {
		return err
	}
	if err := observeSnapshot(); err != nil {
		return err
	}
	if err := s.files.observeAccountAndRoots(s.checked); err != nil {
		return err
	}
	if err := s.invocation.Observe(s.ctx); err != nil {
		return err
	}
	if err := s.observeManager(); err != nil {
		return err
	}
	if err := s.invocation.Observe(s.ctx); err != nil {
		return err
	}
	if err := observeSnapshot(); err != nil {
		return err
	}
	if err := observeFiles(s.ctx); err != nil {
		return err
	}
	if err := s.files.observeAccountAndRoots(s.checked); err != nil {
		return err
	}
	return s.ctx.Err()
}

func (s *startupInspection) observeManager() error {
	observation, err := systemd.ObserveStartup(s.ctx)
	if err != nil {
		return err
	}
	if s.manager != "" && s.manager != observation.Manager {
		return ErrBinding
	}
	for _, round := range observation.Rounds {
		checkProgram, manifest, pin, err := platformCheckBinding(round.EndpointService)
		if err != nil {
			return err
		}
		if err := verifyPlatformRunningEndpointProperties(observation.Version, round.EndpointUnit, round.EndpointService, s.checked.request, s.checked.selected.GenerationDigest, uint32(os.Getpid()), s.id, checkProgram, manifest, pin); err != nil {
			return err
		}
		if err := verifyBoundActivationSocket(round.ReaderUnit, round.ReaderSocket, "reader"); err != nil {
			return err
		}
		if err := verifyBoundActivationSocket(round.PublisherUnit, round.PublisherSocket, "publisher"); err != nil {
			return err
		}
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	s.manager = observation.Manager
	return s.ctx.Err()
}

func (s *startupInspection) close() error {
	if s == nil {
		return nil
	}
	if s.invocation != nil {
		s.terminal = errors.Join(s.terminal, s.invocation.Close())
		s.invocation = nil
	}
	if s.snapshot != nil {
		s.terminal = errors.Join(s.terminal, s.snapshot.Close())
		s.snapshot = nil
	}
	if s.root != nil {
		s.terminal = errors.Join(s.terminal, s.root.Close())
		s.root = nil
	}
	return s.terminal
}
