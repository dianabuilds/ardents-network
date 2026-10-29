package forwarding

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// runtimeSelection independently owns its entire read-only file observation.
// The shared fd/read machinery is instantiated afresh, without a journal fd,
// root writer lease, creator Inputs or Completed handle. No semantic contents
// are opened. Installation's Startup must separately observe manager/process
// and complete its original root-peer exchange before exposing runtime inputs.
type runtimeSelection struct {
	files   *completedFiles
	binding binding
	plan    Plan
	// Exact immutable directories must remain read-only in this invocation's
	// mount namespace; semantic roots must remain native and writable.
	immutable []string
	writable  []string
}

func readRuntimeSelection(ctx context.Context, path string) (result *runtimeSelection, returned error) {
	if ctx == nil || runtime.GOARCH != "amd64" || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() || !validPath(path) {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s := &runtimeSelection{files: &completedFiles{ctx: ctx, files: map[string]*initialFile{}, dirs: map[string]*inputDirectory{}}}
	f := s.files
	defer func() {
		if returned != nil {
			returned = errors.Join(returned, f.close())
			result = nil
		}
	}()
	bound, err := f.read(path, 32<<10, 0644, 0, 0)
	if err != nil {
		return nil, err
	}
	selected, err := decodeSelection(bound)
	b := selected.binding
	if err != nil || path != filepath.Join(b.Request.InstallationRoot, "binding.json") || b.UID != uint32(os.Geteuid()) || b.GID != uint32(os.Getegid()) {
		return nil, errors.Join(ErrBinding, err)
	}
	s.binding = b
	f.clock, f.pending = b.Request.Clock, b.Request.InstallationRoot+".forwarding-initial-pending"
	f.sealed, f.credential = filepath.Join(b.Request.InstallationRoot, "generations", b.Generation), b.Request.CredentialDirectory
	if _, err := f.parents(f.pending); err != nil {
		return nil, err
	}
	if err := f.absent(f.pending); err != nil {
		return nil, err
	}
	if err := f.absent(replacementPending(b.Request.InstallationRoot)); err != nil {
		return nil, err
	}
	f.absentPaths = append(f.absentPaths, replacementPending(b.Request.InstallationRoot))
	for _, dir := range []struct {
		path string
		mode os.FileMode
	}{
		{b.Request.InstallationRoot, 0755}, {filepath.Dir(f.sealed), 0755}, {f.sealed, 0555}, {f.credential, 0755},
	} {
		if err := s.pinImmutableDirectory(dir.path, dir.mode); err != nil {
			return nil, err
		}
		s.immutable = append(s.immutable, dir.path)
	}
	s.writable = []string{b.Request.StateRoot, b.Request.ParticipationRoot, b.Request.SpendRoot, b.Request.HostingRoot}
	for _, path := range s.writable {
		if err := f.directory(path, 0700, b.UID, b.GID); err != nil {
			return nil, err
		}
	}
	public := make(map[string][]byte, 12)
	for _, name := range forwardingGenerationNames() {
		maximum, mode := int64(1<<20), os.FileMode(0444)
		switch name {
		case programName:
			maximum, mode = 64<<20, 0555
		case "binding.json":
			maximum = 32 << 10
		case "root.json", "timestamp.json", "snapshot.json", "targets.json":
		default:
			maximum = 16 << 10
		}
		public[name], err = f.read(filepath.Join(f.sealed, name), maximum, mode, 0, 0)
		if err != nil {
			return nil, err
		}
	}
	s.plan, err = validateSelectedGeneration(b, bound, public)
	if err != nil {
		return nil, err
	}
	certificate, err := f.read(filepath.Join(f.credential, "certificate.pem"), 64<<10, 0444, 0, 0)
	if err != nil || digest(certificate) != b.CertificateSHA256 {
		return nil, errors.Join(ErrBinding, err)
	}
	key, err := f.read(filepath.Join(f.credential, "private-key.pem"), 16<<10, 0640, 0, b.GID)
	if err != nil {
		return nil, err
	}
	pair, err := tls.X509KeyPair(certificate, key)
	if err != nil {
		return nil, err
	}
	private, ok := pair.PrivateKey.(ed25519.PrivateKey)
	if !ok || len(private) != ed25519.PrivateKeySize {
		return nil, ErrBinding
	}
	clear(private)
	pair.PrivateKey = nil
	unit, err := f.read("/etc/systemd/system/ardents-forwarding.service", 16<<10, 0644, 0, 0)
	if err != nil || !bytes.Equal(unit, public["rendered-forwarding.service"]) {
		return nil, errors.Join(ErrBinding, err)
	}
	if _, err := f.parents(f.clock); err != nil {
		return nil, err
	}
	return s, s.check()
}

func (s *runtimeSelection) pinImmutableDirectory(path string, mode os.FileMode) error {
	f := s.files
	parent, err := f.parents(path)
	if err != nil {
		return err
	}
	d := f.dirs[path]
	if d == nil {
		file, err := openInputAt(parent, filepath.Base(path), true)
		if err != nil {
			return err
		}
		d = &inputDirectory{path: path, name: filepath.Base(path), file: file, parent: parent}
		f.dirs[path], f.dirOrder = d, append(f.dirOrder, d)
		d.info, err = file.Stat()
		if err != nil {
			return err
		}
	}
	if !nodeAccess(d.info, mode, 0, 0, true) {
		return ErrBinding
	}
	return runtimeDirectoryMount(d, true)
}

// The bool describes a measured mount flag, never authority or a role choice.
// Only this closed runtime inventory calls the physical check; root Completed
// keeps its separate writable native-directory admission unchanged.
func runtimeDirectoryMount(d *inputDirectory, readOnly bool) (returned error) {
	if d == nil || d.file == nil {
		return ErrInput
	}
	var fs syscall.Statfs_t
	err := syscall.Fstatfs(int(d.file.Fd()), &fs)
	if err != nil || fs.Type != 0xef53 || (fs.Flags&1 != 0) != readOnly {
		return errors.Join(ErrBinding, err)
	}
	// A new bind mount can name the same inode with different access flags.
	// The retained fd's old mount alone cannot attest the current pathname.
	named, err := os.OpenFile(d.path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, named.Close()) }()
	info, err := named.Stat()
	fsErr := syscall.Fstatfs(int(named.Fd()), &fs)
	if err != nil || fsErr != nil || d.info == nil || !os.SameFile(d.info, info) || fs.Type != 0xef53 || (fs.Flags&1 != 0) != readOnly {
		return errors.Join(ErrBinding, err, fsErr)
	}
	return nil
}

func (s *runtimeSelection) check() (returned error) {
	if s == nil || s.files == nil {
		return ErrInput
	}
	f := s.files
	if err := f.begin(); err != nil {
		return err
	}
	defer f.uses.Done()
	f.ioMu.Lock()
	defer f.ioMu.Unlock()
	defer func() { f.record(returned) }()
	if err := f.check(); err != nil {
		return err
	}
	for _, path := range s.immutable {
		if err := runtimeDirectoryMount(f.dirs[path], true); err != nil {
			return err
		}
	}
	for _, path := range s.writable {
		if err := runtimeDirectoryMount(f.dirs[path], false); err != nil {
			return err
		}
	}
	if err := observeNodePrincipal(f.ctx, s.binding.UID, s.binding.GID); err != nil {
		return err
	}
	return errors.Join(f.check(), f.local())
}

func (s *runtimeSelection) close() error {
	if s == nil {
		return nil
	}
	return s.files.close()
}
