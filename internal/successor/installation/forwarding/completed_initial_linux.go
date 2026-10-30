package forwarding

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Completed retains independently reopened initial and selected-generation
// lineage histories and native files.
// It supplies no creator custody, current duty, Start, retirement or Startup
// permission. Fresh artifact authorization is a separate checked operation.
type Completed struct{ original *completedInitial }

type completedInitial struct {
	files          *completedFiles
	binding        binding
	plan           Plan
	initial        generationSelection
	selections     map[string]generationSelection
	selectionOrder []generationSelection
	hostingPolicy  string
	reloads        map[string]*replacementNativeProof
}

// ReadCompleted is root-only and noncreating. It requires the exact completed
// initial transcript, absent pending marker and its own exclusive journal lease;
// a partial/uncertain prefix cannot be adopted or continued through this opener.
// No State, participation, spend or Hosting content is read or initialized.
func ReadCompleted(ctx context.Context, path string) (Completed, error) {
	return readCompleted(ctx, path, nil)
}

func readCompleted(ctx context.Context, path string, observe func(string, *os.File)) (result Completed, returned error) {
	return readCompletedObservation(ctx, path, observe, nil)
}

// A replacement reader obtains every descriptor and exclusive lease anew after
// the writer and predecessor readers close. The expected pending inode restricts
// this private readback; it grants no ordinary completion or continuation.
func readCompletedPending(ctx context.Context, path string, pending *initialFile) (Completed, error) {
	if pending == nil {
		return Completed{}, ErrInput
	}
	return readCompletedObservation(ctx, path, nil, pending)
}

func readCompletedObservation(ctx context.Context, path string, observe func(string, *os.File), pending *initialFile) (result Completed, returned error) {
	return readCompletedWithRecovery(ctx, path, observe, pending, nil)
}

// Actual recovery readback takes new leases only after the original and suffix
// writers close. The closed publication identifies its own two present guards.
func readCompletedWithRecovery(ctx context.Context, path string, observe func(string, *os.File), pending *initialFile, recovery *recoveryPublication) (result Completed, returned error) {
	return readCompletedInputs(ctx, path, observe, pending, recovery, nil)
}

// Only the actual recovery operation supplies its own freshly opened original
// lease. All file observations are independent; this view is never public
// completed custody and cannot be returned as permission to another operation.
func readCompletedInterrupted(ctx context.Context, path string, original *recoveryOriginal) (Completed, error) {
	if original == nil {
		return Completed{}, ErrInput
	}
	return readCompletedInputs(ctx, path, nil, nil, nil, original)
}

func readCompletedInputs(ctx context.Context, path string, observe func(string, *os.File), pending *initialFile, recovery *recoveryPublication, interrupted *recoveryOriginal) (result Completed, returned error) {
	if ctx == nil || os.Geteuid() != 0 || !validPath(path) {
		return result, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	s := &completedFiles{ctx: ctx, files: map[string]*initialFile{}, dirs: map[string]*inputDirectory{}, observe: observe, interrupted: interrupted, recoveryReadback: recovery}
	c := &completedInitial{files: s}
	stage := "selected binding"
	defer func() {
		if returned != nil {
			returned = errors.Join(fmt.Errorf("forwarding completed read %s: %w", stage, returned), s.close())
			result = Completed{}
		}
	}()
	bound, err := s.read(path, 32<<10, 0644, 0, 0)
	if err != nil {
		return result, err
	}
	selected, err := decodeSelection(bound)
	b := selected.binding
	if err != nil || path != filepath.Join(b.Request.InstallationRoot, "binding.json") {
		return result, errors.Join(ErrBinding, err)
	}
	c.binding = b
	if interrupted != nil {
		matched := interrupted.cut.Boundary.Operation == "replacement" && interrupted.cut.Boundary.Journal == replacementJournal(b.Request.InstallationRoot, b.Generation)
		if interrupted.cut.Boundary.Operation == "retirement" || interrupted.cut.Boundary.Operation == "start" {
			for n := attemptNumber(0); n < attemptLimit; n++ {
				journal := n.retirementJournal(b.Request.InstallationRoot)
				if interrupted.cut.Boundary.Operation == "start" {
					journal = n.journal(b.Request.InstallationRoot)
				}
				if interrupted.cut.Boundary.Journal == journal {
					matched = true
					break
				}
			}
		}
		if !matched || !bytes.Equal(interrupted.cut.Boundary.Binding, selected.raw) {
			return result, ErrBinding
		}
		if err := interrupted.check(); err != nil {
			return result, err
		}
	}
	s.clock, s.pending = b.Request.Clock, b.Request.InstallationRoot+".forwarding-initial-pending"
	s.sealed = filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	s.credential = b.Request.CredentialDirectory
	if _, err := s.parents(s.pending); err != nil {
		return result, err
	}
	stage = "initial pending publication"
	if recovery == nil || recovery.completion.Operation != "initial" {
		if err := s.absent(s.pending); err != nil {
			return result, err
		}
	}
	otherPending := replacementPending(b.Request.InstallationRoot)
	stage = "replacement pending custody"
	if interrupted != nil && interrupted.cut.Boundary.Operation == "replacement" {
		raw, err := s.read(otherPending, 128, 0600, 0, 0)
		if err != nil || !bytes.Equal(raw, replacementPendingBytes(selected.raw)) {
			return result, errors.Join(ErrBinding, err)
		}
	} else if pending == nil {
		if err := s.absent(otherPending); err != nil {
			return result, err
		}
		s.absentPaths = append(s.absentPaths, otherPending)
	} else {
		if selected.replacement == nil || pending.path != otherPending || pending.info == nil {
			return result, ErrBinding
		}
		raw, err := s.read(otherPending, 128, 0600, 0, 0)
		if err != nil || !bytes.Equal(raw, replacementPendingBytes(selected.raw)) || !sameInputFile(pending.info, s.files[otherPending].info) {
			return result, errors.Join(ErrBinding, err)
		}
	}
	journal := b.Request.InstallationRoot + ".forwarding-initial-journal"
	stage = "initial journal custody"
	s.journal, err = s.retain(journal, 4<<20, 0600, 0, 0)
	if err != nil {
		return result, err
	}
	if err := syscall.Flock(int(s.journal.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return result, err
	}
	if _, err := s.fill(s.journal, 4<<20); err != nil {
		return result, err
	}
	stage = "original directory custody"
	for _, d := range []struct {
		path     string
		mode     os.FileMode
		uid, gid uint32
	}{
		{b.Request.InstallationRoot, 0755, 0, 0}, {filepath.Dir(s.sealed), 0755, 0, 0}, {s.sealed, 0555, 0, 0},
		{b.Request.CredentialDirectory, 0755, 0, 0}, {b.History, 0700, 0, 0},
		{b.Request.StateRoot, 0700, b.UID, b.GID}, {b.Request.ParticipationRoot, 0700, b.UID, b.GID},
		{b.Request.SpendRoot, 0700, b.UID, b.GID}, {b.Request.HostingRoot, 0700, b.UID, b.GID},
	} {
		if err := s.directory(d.path, d.mode, d.uid, d.gid); err != nil {
			return result, err
		}
	}
	stage = "sealed generation lineage"
	if err := c.readSelections(selected); err != nil {
		return result, err
	}
	get := func(name string) []byte { return s.files[filepath.Join(s.sealed, name)].body }
	stage = "credential pair"
	certificate, err := s.read(filepath.Join(b.Request.CredentialDirectory, "certificate.pem"), 64<<10, 0444, 0, 0)
	if err != nil {
		return result, err
	}
	key, err := s.read(filepath.Join(b.Request.CredentialDirectory, "private-key.pem"), 16<<10, 0640, 0, b.GID)
	if err != nil {
		return result, err
	}
	pair, err := tls.X509KeyPair(certificate, key)
	if err != nil {
		return result, err
	}
	private, ok := pair.PrivateKey.(ed25519.PrivateKey)
	if !ok || len(private) != ed25519.PrivateKeySize {
		return result, ErrBinding
	}
	clear(private)
	pair.PrivateKey = nil
	stage = "selected fixed unit and clock"
	unit, err := s.read("/etc/systemd/system/ardents-forwarding.service", 16<<10, 0644, 0, 0)
	if err != nil || !bytes.Equal(unit, get("rendered-forwarding.service")) {
		return result, errors.Join(ErrBinding, err)
	}
	if _, err := s.parents(s.clock); err != nil {
		return result, err
	}
	stage = "original initial history"
	hashes := c.initialHashes(path)
	var initialRecovery *recoveryPublication
	if recovery != nil && recovery.completion.Operation == "initial" {
		initialRecovery = recovery
	}
	initialRaw, err := c.readRecoveredInitial(initialRecovery)
	if err != nil {
		return result, err
	}
	records, err := decodeInitialRecords(initialRaw, c.initial.binding, hashes)
	if err != nil {
		return result, err
	}
	stage = "original initial native inventory"
	for path, record := range records {
		if path == s.pending {
			if s.recovered != nil && s.recovered.originalPending != nil {
				native, ok := infoSys(s.recovered.originalPending.info)
				if !ok || uint64(native.Dev) != record.Device || native.Ino != record.Inode {
					return result, ErrBinding
				}
			}
			continue
		}
		var info os.FileInfo
		if file := s.files[path]; file != nil {
			info = file.info
		} else if dir := s.dirs[path]; dir != nil {
			info = dir.info
		}
		if info == nil || !nodeAccess(info, os.FileMode(record.Mode), record.UID, record.GID, info.IsDir()) {
			return result, ErrBinding
		}
		native := info.Sys().(*syscall.Stat_t)
		if uint64(native.Dev) != record.Device || native.Ino != record.Inode {
			return result, ErrBinding
		}
	}
	stage = "replacement native history"
	if err := c.readReplacementHistory(); err != nil {
		return result, err
	}
	stage = "invocation history"
	s.invocations = &invocationChain{ctx: ctx, owner: c}
	if err := s.invocations.readChain(path); err != nil {
		return result, err
	}
	stage = "retained native context"
	if err := c.openRecoveredContext(); err != nil {
		return result, err
	}
	result = Completed{original: c}
	stage = "final retained observation"
	return result, result.Check()
}

func (h Completed) operation(check func(*completedInitial) error) (returned error) {
	c := h.original
	if c == nil || c.files == nil {
		return ErrInput
	}
	s := c.files
	if err := s.begin(); err != nil {
		return err
	}
	defer s.uses.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	defer func() { s.record(returned) }()
	if err := s.local(); err != nil {
		return err
	}
	if check != nil {
		if err := check(c); err != nil {
			return err
		}
	}
	if err := s.check(); err != nil {
		return err
	}
	if err := observeNodePrincipal(s.ctx, c.binding.UID, c.binding.GID); err != nil {
		return err
	}
	return errors.Join(s.check(), s.local())
}

// Check retains only the original physical observation and current principal.
// It grants no fresh Release, completed retirement, State or runtime authority.
func (h Completed) Check() error { return h.operation(nil) }

// CheckHistoryPath admits only the already bound retained Release directory;
// it opens no verifier and grants no unexpired artifact or runtime authority.
func (h Completed) CheckHistoryPath(history string) error {
	return h.operation(func(c *completedInitial) error {
		if !validPath(history) || history != c.binding.History {
			return ErrBinding
		}
		return nil
	})
}

func (h Completed) IndependentPaths(distribution, history string) error {
	return h.operation(func(c *completedInitial) error {
		if !validPath(distribution) || !validPath(history) || history != c.binding.History || contains(distribution, history) || contains(history, distribution) {
			return ErrBinding
		}
		paths := c.binding.Request.ownedPaths()
		for _, path := range paths {
			if contains(distribution, path) || contains(path, distribution) {
				return ErrBinding
			}
		}
		return nil
	})
}

// CheckAuthorization matches a separately fresh genuine pair to the same
// installed targets. Fresh metadata may advance Release floors without rewriting
// historical sealed initial metadata. The verifier retains its own live root;
// this observation holds only an independent descriptor of its original directory.
func (h Completed) CheckAuthorization(v *release.Verifier, a infrastructure.ForwardingAuthorization) error {
	return h.operation(func(c *completedInitial) error {
		if v == nil {
			return ErrInput
		}
		s := c.files
		if err := errors.Join(v.CheckHistoryPath(s.ctx, c.binding.History), a.Check(s.ctx, v)); err != nil {
			return err
		}
		for name, raw := range map[string][]byte{programName: a.Program(), infrastructure.ForwardingDescriptorName: a.Descriptor(), infrastructure.ForwardingTemplateName: a.Template()} {
			file := s.files[filepath.Join(s.sealed, name)]
			if file == nil || !bytes.Equal(file.body, raw) {
				return ErrBinding
			}
		}
		return errors.Join(a.Check(s.ctx, v), v.CheckHistoryPath(s.ctx, c.binding.History))
	})
}

func (h Completed) Close() error {
	if h.original == nil {
		return nil
	}
	return h.original.files.close()
}
