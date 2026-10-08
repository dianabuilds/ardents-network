package installation

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/generation"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/process"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// Startup retains independently observed installed bytes, the original service
// process and manager after the root exchange. It conveys no Release, Job or
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

// OpenStartup observes this actual non-root installed invocation, joins the
// bounded original root exchange and reobserves it before yielding any plans.
func OpenStartup(ctx context.Context, directory string) (*Startup, error) {
	inspection, err := openStartupInspection(ctx, directory)
	if err != nil {
		return nil, err
	}
	if err := inspection.awaitRootCompletion(); err != nil {
		return nil, errors.Join(err, inspection.close())
	}
	return &Startup{lifetime: &startupLifetime{inspection: inspection}}, nil
}

// Check reobserves the same retained installed invocation before an effect.
func (s *Startup) Check() error {
	if s == nil || s.lifetime == nil {
		return ErrInput
	}
	s.lifetime.mu.Lock()
	defer s.lifetime.mu.Unlock()
	return s.lifetime.checkLocked()
}

func (s *startupLifetime) checkLocked() error {
	if s.closed || s.inspection == nil {
		return errors.Join(ErrBinding, s.result)
	}
	if s.result != nil {
		return s.result
	}
	s.result = s.inspection.observe()
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
	if err := s.lifetime.checkLocked(); err != nil {
		return nil, nil, err
	}
	headless := bytes.Clone(s.lifetime.inspection.checked.files["headless.json"])
	source := bytes.Clone(s.lifetime.inspection.checked.files["source.json"])
	if err := s.lifetime.checkLocked(); err != nil {
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

// startupInspection owns independent service-account observations before the
// completion handshake. It is not a startup receipt or runtime permit. Root
// retains transition authorization; this owner never opens its writer lease.
// The original caller, process and sealed snapshot live until physical close.
type startupInspection struct {
	ctx                 context.Context
	root                *os.Root
	identity            os.FileInfo
	files               *installedFiles
	checked             inspectedGeneration
	snapshot            *generation.Snapshot
	invocation          *process.Invocation
	id                  [16]byte
	terminal            error
	completion          *completion.Exchange
	completionAttempted bool
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
		directory: directory, gid: native.Gid, files: make(map[string]stagedFile),
		directories: ancestors, mutableDirectories: make(map[string]os.FileInfo),
	}}
	defer func() {
		if returnedErr != nil {
			s.terminal = returnedErr
			returnedErr = s.close()
			result = nil
		}
	}()
	selectedRaw, err := s.files.read(ctx, filepath.Join(directory, "selection.json"), 4<<10, 0640, native.Gid)
	if err != nil {
		return nil, err
	}
	var selected generationSelection
	if decodeCanonical(selectedRaw, 4<<10, &selected) != nil || !canonicalDigest(selected.GenerationDigest) {
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
	if err := s.observe(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *startupInspection) observe() (returnedErr error) {
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
	if err != nil || !sameStagingDirectory(s.identity, current) {
		return errors.Join(ErrBinding, err)
	}
	if err := s.files.observe(s.ctx); err != nil {
		return err
	}
	if err := s.snapshot.Observe(); err != nil {
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
	if err := s.snapshot.Observe(); err != nil {
		return err
	}
	if err := s.files.observe(s.ctx); err != nil {
		return err
	}
	if err := s.files.observeAccountAndRoots(s.checked); err != nil {
		return err
	}
	return s.ctx.Err()
}

func (s *startupInspection) observeManager() error {
	version, err := systemd.Version(s.ctx)
	if err != nil {
		return err
	}
	for round := 0; round < 2; round++ {
		unit, service, err := systemd.Endpoint(s.ctx)
		if err != nil {
			return err
		}
		if err := verifyRunningEndpointProperties(version, unit, service, s.checked.request, s.checked.selected.GenerationDigest, uint32(os.Getpid()), s.id); err != nil {
			return err
		}
		for _, role := range []string{"reader", "publisher"} {
			unit, socket, err := systemd.Activation(s.ctx, role)
			if err != nil {
				return err
			}
			if err := verifyBoundActivationSocket(unit, socket, role); err != nil {
				return err
			}
		}
	}
	return s.ctx.Err()
}

func (s *startupInspection) close() error {
	if s == nil {
		return nil
	}
	if s.completion != nil {
		s.terminal = errors.Join(s.terminal, s.completion.Close())
		s.completion = nil
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

// This joins only the guarded local exchange and final original observations.
// It does not mint a runtime permit or substitute for the actual new consumer.
func (s *startupInspection) awaitRootCompletion() (returnedErr error) {
	if s == nil || s.ctx == nil || s.completionAttempted {
		return ErrInput
	}
	if err := s.observe(); err != nil {
		return err
	}
	s.completionAttempted = true
	defer func() {
		if returnedErr != nil {
			s.terminal = errors.Join(s.terminal, returnedErr)
		}
	}()
	frame, err := installationCompletionFrame(s.checked.selected, s.id)
	if err != nil {
		return err
	}
	// The root successor operation has its own two-minute bound. This local
	// exchange cannot prolong an earlier caller deadline or turn its bounded
	// socket into the post-ACK participant lifetime.
	exchangeContext, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()
	s.completion, err = completion.Connect(exchangeContext, s.files.directory, s.identity, s.checked.binding.GID, [160]byte(frame))
	if err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.completion.Wait(); err != nil {
		return err
	}
	if err := s.completion.Close(); err != nil {
		return err
	}
	s.completion = nil
	// Root cleanup may remove its guard/socket after ACK. The selected bytes,
	// sealed snapshot, self invocation and manager must still be the originals.
	return s.observe()
}
