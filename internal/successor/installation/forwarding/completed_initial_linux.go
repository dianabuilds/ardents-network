package forwarding

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Completed retains an independently reopened initial history and native files.
// It supplies no creator custody, current duty, Start, retirement or Startup
// permission. Fresh artifact authorization is a separate checked operation.
type Completed struct{ original *completedInitial }

type completedInitial struct {
	files   *completedFiles
	binding binding
	plan    Plan
}

// ReadCompleted is root-only and noncreating. It requires the exact completed
// initial transcript, absent pending marker and its own exclusive journal lease;
// a partial/uncertain prefix cannot be adopted or continued through this opener.
// No State, participation, spend or Hosting content is read or initialized.
func ReadCompleted(ctx context.Context, path string) (Completed, error) {
	return readCompleted(ctx, path, nil)
}

func readCompleted(ctx context.Context, path string, observe func(string, *os.File)) (result Completed, returned error) {
	if ctx == nil || os.Geteuid() != 0 || !validPath(path) {
		return result, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	s := &completedFiles{ctx: ctx, files: map[string]*initialFile{}, dirs: map[string]*inputDirectory{}, observe: observe}
	c := &completedInitial{files: s}
	defer func() {
		if returned != nil {
			returned = errors.Join(returned, s.close())
			result = Completed{}
		}
	}()
	bound, err := s.read(path, 32<<10, 0644, 0, 0)
	if err != nil {
		return result, err
	}
	b, err := decodeBinding(bound)
	if err != nil || path != filepath.Join(b.Request.InstallationRoot, "binding.json") {
		return result, errors.Join(ErrBinding, err)
	}
	c.binding = b
	s.clock, s.pending = b.Request.Clock, b.Request.InstallationRoot+".forwarding-initial-pending"
	s.sealed = filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	s.credential = b.Request.CredentialDirectory
	if _, err := s.parents(s.pending); err != nil {
		return result, err
	}
	if err := s.absent(s.pending); err != nil {
		return result, err
	}
	journal := b.Request.InstallationRoot + ".forwarding-initial-journal"
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
	public := make(map[string][]byte, 12)
	for _, name := range forwardingGenerationNames() {
		limit, mode := int64(1<<20), os.FileMode(0444)
		if name == programName {
			limit, mode = 64<<20, 0555
		} else if name == "binding.json" {
			limit = 32 << 10
		} else if name != "root.json" && name != "timestamp.json" && name != "snapshot.json" && name != "targets.json" {
			limit = 16 << 10
		}
		public[name], err = s.read(filepath.Join(s.sealed, name), limit, mode, 0, 0)
		if err != nil {
			return result, err
		}
	}
	get := func(name string) []byte { return s.files[filepath.Join(s.sealed, name)].body }
	c.plan, err = validateSelectedGeneration(b, bound, public)
	if err != nil {
		return result, err
	}
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
	unit, err := s.read("/etc/systemd/system/ardents-forwarding.service", 16<<10, 0644, 0, 0)
	if err != nil || !bytes.Equal(unit, get("rendered-forwarding.service")) {
		return result, errors.Join(ErrBinding, err)
	}
	if _, err := s.parents(s.clock); err != nil {
		return result, err
	}
	hashes := make(map[string]string, len(s.files))
	for path, file := range s.files {
		if path != journal {
			hashes[path] = digest(file.body)
		}
	}
	hashes[s.pending] = digest([]byte("ardents-forwarding-initial-pending-v1\n" + digest(bound) + "\n"))
	records, err := decodeInitialRecords(s.journal.body, b, hashes)
	if err != nil {
		return result, err
	}
	for path, record := range records {
		if path == s.pending {
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
	result = Completed{original: c}
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
