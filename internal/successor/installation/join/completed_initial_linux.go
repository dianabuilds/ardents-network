package join

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Completed retains independent read-only native observations and the original
// initial journal exclusion. It grants no Start, semantic initialization,
// replacement, recovery or runtime authority. Copies share one Close result.
type Completed struct{ original *completedInitial }

type completedInitial struct {
	ctx             context.Context
	mu, ioMu        sync.Mutex
	uses            sync.WaitGroup
	once            sync.Once
	closed          bool
	failure         error
	binding         binding
	parents         *nativeDirectories
	files           map[string]*initialFile
	fileOrder       []*initialFile
	dirs            []*inputDirectory
	journal         *initialFile
	absent          []string
	sealed          string
	hostingBirths   *initialBirths
	hostingPrepared bool
	// Failure-only causal control after genuine original file observation.
	observe func(*os.File)
}

var errCompletedClosed = errors.New("JOIN installation: completed observation closed")

// ReadCompleted opens the whole initial history after pending publication. Its
// own exclusive lease precedes transcript use. Account databases are checked as
// current principal facts, not frozen postimages; semantic contents are unopened.
func ReadCompleted(ctx context.Context, path string) (result Completed, returned error) {
	if ctx == nil || os.Geteuid() != 0 || !validPath(path) {
		return result, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	c := &completedInitial{ctx: ctx, parents: &nativeDirectories{ctx: ctx, byPath: map[string]*inputDirectory{}}, files: map[string]*initialFile{}}
	defer func() {
		if returned != nil {
			c.failure = returned
			returned = errors.Join(returned, (Completed{c}).Close())
			result = Completed{}
		}
	}()
	selected, err := c.read(path, 32<<10, 0644, 0, 0)
	if err != nil {
		return result, err
	}
	b, err := decodeBinding(selected.body)
	if err != nil || path != filepath.Join(b.Request.InstallationRoot, "binding.json") {
		return result, errors.Join(ErrBinding, err)
	}
	c.binding = b
	r := b.Request
	c.sealed = filepath.Join(r.InstallationRoot, "generations", b.Generation)
	journal, err := c.read(r.InstallationRoot+".join-initial-journal", 4<<20, 0600, 0, 0)
	if err != nil {
		return result, err
	}
	c.journal = journal
	if err := syscall.Flock(int(journal.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return result, err
	}
	if err := observeInitialFile(journal); err != nil {
		return result, err
	}
	// No other operation exists in this completed-initial boundary. Retained
	// future journal names are checked, never interpreted as successful history.
	hostingDirectory, hostingJournal, _ := hostingPreparationPaths(r.InstallationRoot)
	hostingPresent := false
	for _, reserved := range r.reservedSiblingPaths() {
		if reserved == journal.path {
			continue
		}
		if _, err := c.parents.parents(reserved); err != nil {
			return result, err
		}
		if _, err := os.Lstat(reserved); !errors.Is(err, os.ErrNotExist) {
			if err == nil && (reserved == hostingDirectory || reserved == hostingJournal) {
				hostingPresent = true
				continue // Independent typed transcript admission below, never presence authority.
			}
			return result, errors.Join(ErrBinding, err)
		}
		c.absent = append(c.absent, reserved)
	}
	for _, d := range []struct {
		path     string
		mode     os.FileMode
		uid, gid uint32
	}{
		{r.InstallationRoot, 0755, 0, 0}, {filepath.Dir(c.sealed), 0755, 0, 0}, {c.sealed, 0555, 0, 0},
		{r.CredentialDirectory, 0755, 0, 0}, {b.History, 0700, 0, 0},
		{r.StateRoot, 0700, b.UID, b.GID}, {r.ParticipationRoot, 0700, b.UID, b.GID},
		{r.SpendRoot, 0700, b.UID, b.GID}, {r.HostingRoot, 0700, b.UID, b.GID},
	} {
		if err := c.directory(d.path, d.mode, d.uid, d.gid); err != nil {
			return result, err
		}
	}
	artifacts := map[string][]byte{}
	for _, name := range []string{programName, infrastructure.JoinTemplateName, infrastructure.JoinDescriptorName, "infrastructure-enrollment.json", "root.json", "timestamp.json", "snapshot.json", "targets.json", "join-request.json", "join-plan.json", "rendered-join.service", "binding.json"} {
		mode := os.FileMode(0444)
		maximum := int64(1 << 20)
		if name == programName {
			mode = 0555
			maximum = 64 << 20
		}
		file, err := c.read(filepath.Join(c.sealed, name), maximum, mode, 0, 0)
		if err != nil {
			return result, err
		}
		artifacts[name] = file.body
	}
	if _, err := validateInitialGeneration(b, selected.body, artifacts); err != nil {
		return result, err
	}
	cert, err := c.read(filepath.Join(r.CredentialDirectory, "certificate.pem"), 8192, 0444, 0, 0)
	if err != nil {
		return result, err
	}
	key, err := c.read(filepath.Join(r.CredentialDirectory, "private-key.pem"), 8192, 0640, 0, b.GID)
	if err != nil {
		return result, err
	}
	pair, err := tls.X509KeyPair(cert.body, key.body)
	if err != nil {
		return result, err
	}
	private, ok := pair.PrivateKey.(ed25519.PrivateKey)
	if ok {
		clear(private)
	}
	pair.PrivateKey = nil
	if !ok || len(private) != ed25519.PrivateKeySize || digest(cert.body) != b.CertificateSHA256 {
		return result, ErrBinding
	}
	unit, err := c.read("/etc/systemd/system/ardents-join.service", 16<<10, 0644, 0, 0)
	if err != nil || !bytes.Equal(unit.body, artifacts["rendered-join.service"]) {
		return result, errors.Join(ErrBinding, err)
	}
	hashes := map[string]string{}
	for path, file := range c.files {
		hashes[path] = digest(file.body)
	}
	delete(hashes, journal.path)
	hashes[r.InstallationRoot+".join-initial-pending"] = digest([]byte("ardents-join-initial-pending-v1\n" + digest(selected.body) + "\n"))
	records, err := decodeInitialRecords(journal.body, b, hashes)
	if err != nil {
		return result, err
	}
	for path, record := range records {
		if path == r.InstallationRoot+".join-initial-pending" {
			continue
		}
		var info os.FileInfo
		if file := c.files[path]; file != nil {
			info = file.info
		}
		for _, d := range c.dirs {
			if d.path == path {
				info = d.info
			}
		}
		native, ok := infoSys(info)
		if !ok || !nodeAccess(info, os.FileMode(record.Mode), record.UID, record.GID, info.IsDir()) || uint64(native.Dev) != record.Device || native.Ino != record.Inode {
			return result, ErrBinding
		}
	}
	if hostingPresent {
		if err := c.readHostingPreparation(); err != nil {
			return result, err
		}
	}
	result = Completed{c}
	return result, result.Check()
}

func (c *completedInitial) read(path string, maximum int64, mode os.FileMode, uid, gid uint32) (*initialFile, error) {
	if c.files[path] != nil {
		return nil, ErrInput
	}
	parent, err := c.parents.parents(path)
	if err != nil {
		return nil, err
	}
	return c.readAt(parent, path, maximum, mode, uid, gid)
}

// A runtime-owned semantic root is retained with its recorded birth and numeric
// principal, never treated as a trusted root-owned ancestor or adopted by name.
func (c *completedInitial) readAt(parent *inputDirectory, path string, maximum int64, mode os.FileMode, uid, gid uint32) (*initialFile, error) {
	if parent == nil || c.files[path] != nil || parent.path != filepath.Dir(path) {
		return nil, ErrInput
	}
	f, err := openInputAt(parent, filepath.Base(path), false)
	if err != nil {
		return nil, err
	}
	original := &initialFile{path: path, file: f}
	c.files[path] = original
	c.fileOrder = append(c.fileOrder, original)
	original.info, err = f.Stat()
	var storage syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(f.Fd()), &storage)
	if err != nil || fsErr != nil || storage.Type != 0xef53 || !nodeAccess(original.info, mode, uid, gid, false) || (maximum != 0 && original.info.Size() < 1) || original.info.Size() > maximum {
		return nil, errors.Join(ErrBinding, err, fsErr)
	}
	original.body, err = io.ReadAll(io.LimitReader(f, maximum+1))
	if err != nil || int64(len(original.body)) != original.info.Size() {
		return nil, errors.Join(ErrBinding, err)
	}
	return original, observeInitialFile(original)
}

func (c *completedInitial) directory(path string, mode os.FileMode, uid, gid uint32) error {
	parent, err := c.parents.parents(path)
	if err != nil {
		return err
	}
	f, err := openInputAt(parent, filepath.Base(path), true)
	if err != nil {
		return err
	}
	d := &inputDirectory{path: path, name: filepath.Base(path), file: f, parent: parent}
	c.dirs = append(c.dirs, d)
	d.info, err = f.Stat()
	var storage syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(f.Fd()), &storage)
	if err != nil || fsErr != nil || storage.Type != 0xef53 || storage.Flags&1 != 0 || !nodeAccess(d.info, mode, uid, gid, true) {
		return errors.Join(ErrBinding, err, fsErr)
	}
	return c.ctx.Err()
}

func (c *completedInitial) local() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failure != nil {
		return c.failure
	}
	if c.closed {
		return errCompletedClosed
	}
	return c.ctx.Err()
}

func (h Completed) operation(effect func(*completedInitial) error) (returned error) {
	c := h.original
	if c == nil {
		return ErrInput
	}
	c.mu.Lock()
	if c.failure != nil {
		err := c.failure
		c.mu.Unlock()
		return err
	}
	if c.closed {
		c.mu.Unlock()
		return errCompletedClosed
	}
	c.uses.Add(1)
	c.mu.Unlock()
	defer c.uses.Done()
	c.ioMu.Lock()
	defer c.ioMu.Unlock()
	defer func() {
		if returned != nil && returned != errCompletedClosed {
			c.mu.Lock()
			if c.failure == nil {
				c.failure = returned
			}
			c.mu.Unlock()
		}
	}()
	if err := c.check(); err != nil {
		return err
	}
	if effect != nil {
		if err := effect(c); err != nil {
			return err
		}
	}
	return c.check()
}

func (c *completedInitial) check() error {
	if err := c.local(); err != nil {
		return err
	}
	if err := c.checkHostingInventory(); err != nil {
		return err
	}
	if err := c.parents.links(); err != nil {
		return err
	}
	for _, f := range c.fileOrder {
		if err := observeInitialFile(f); err != nil {
			return err
		}
		if c.observe != nil {
			c.observe(f.file)
		}
	}
	for _, d := range c.dirs {
		info, err := d.file.Stat()
		named, nameErr := os.Lstat(d.path)
		st, _ := infoSys(d.info)
		if err != nil || nameErr != nil || st == nil || !os.SameFile(d.info, info) || !os.SameFile(d.info, named) || !nodeAccess(info, d.info.Mode().Perm(), st.Uid, st.Gid, true) || !nodeAccess(named, d.info.Mode().Perm(), st.Uid, st.Gid, true) {
			return errors.Join(ErrBinding, err, nameErr)
		}
	}
	if c.hostingBirths != nil {
		if err := c.hostingBirths.check(); err != nil {
			return err
		}
	}
	for _, path := range c.absent {
		if c.hostingOwns(path) {
			continue
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	// This opener accepts only the complete initial namespace. Later operations
	// need their own typed journals and reader extension, never unknown files.
	for directory, wanted := range map[string]map[string]bool{
		c.binding.Request.InstallationRoot:    {"binding.json": true, "generations": true},
		filepath.Dir(c.sealed):                {c.binding.Generation: true},
		c.binding.Request.CredentialDirectory: {"certificate.pem": true, "private-key.pem": true},
	} {
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != len(wanted) {
			return errors.Join(ErrBinding, err)
		}
		for _, entry := range entries {
			if !wanted[entry.Name()] {
				return ErrBinding
			}
		}
	}
	entries, err := os.ReadDir(c.sealed)
	if err != nil || len(entries) != 12 {
		return errors.Join(ErrBinding, err)
	}
	for _, entry := range entries {
		if c.files[filepath.Join(c.sealed, entry.Name())] == nil {
			return ErrBinding
		}
	}
	if err := observeNodePrincipal(c.ctx, c.binding.UID, c.binding.GID); err != nil {
		return err
	}
	return c.local()
}

func (h Completed) Check() error { return h.operation(nil) }

// CheckAuthorization matches fresh permission against historical payloads;
// advancing metadata never replaces the original inventory or history bytes.
func (h Completed) CheckAuthorization(v *release.Verifier, a infrastructure.JoinAuthorization) error {
	return h.operation(func(c *completedInitial) error {
		if v == nil {
			return ErrInput
		}
		if err := errors.Join(a.Check(c.ctx, v), v.CheckHistoryPath(c.ctx, c.binding.History)); err != nil {
			return err
		}
		for name, raw := range map[string][]byte{programName: a.Program(), infrastructure.JoinDescriptorName: a.Descriptor(), infrastructure.JoinTemplateName: a.Template()} {
			if !bytes.Equal(c.files[filepath.Join(c.sealed, name)].body, raw) {
				return ErrBinding
			}
		}
		return errors.Join(a.Check(c.ctx, v), v.CheckHistoryPath(c.ctx, c.binding.History))
	})
}

// IndependentPaths checks declarations before opening the separate verifier.
func (h Completed) IndependentPaths(distribution, history string) error {
	return h.operation(func(c *completedInitial) error {
		if !validPath(distribution) || history != c.binding.History || contains(distribution, history) || contains(history, distribution) {
			return ErrBinding
		}
		for _, path := range c.binding.Request.ownedPaths() {
			if contains(distribution, path) || contains(path, distribution) {
				return ErrBinding
			}
		}
		return nil
	})
}

func (h Completed) Close() error {
	c := h.original
	if c == nil {
		return nil
	}
	c.once.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
		c.uses.Wait()
		var physical error
		for i := len(c.fileOrder) - 1; i >= 0; i-- {
			f := c.fileOrder[i]
			if f == c.journal {
				continue
			}
			physical = errors.Join(physical, f.file.Close())
			clear(f.body)
		}
		for i := len(c.dirs) - 1; i >= 0; i-- {
			physical = errors.Join(physical, c.dirs[i].file.Close())
		}
		physical = errors.Join(physical, c.parents.close())
		// The lease is last: an observer still closing cannot admit replacement.
		if c.journal != nil {
			physical = errors.Join(physical, c.journal.file.Close())
			clear(c.journal.body)
		}
		c.mu.Lock()
		c.failure = errors.Join(c.failure, physical, c.ctx.Err())
		c.mu.Unlock()
	})
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.failure
}

func (c *completedInitial) directoryAt(path string) *inputDirectory {
	for _, d := range c.dirs {
		if d.path == path {
			return d
		}
	}
	return nil
}

// Only this operation's actual born originals may replace an absence check.
// Paths, complete-looking bytes and a foreign partial preparation grant none.
func (c *completedInitial) hostingOwns(path string) bool {
	j := c.hostingBirths
	if j == nil {
		return false
	}
	directory, journal, pending := hostingPreparationPaths(c.binding.Request.InstallationRoot)
	if path == journal {
		return j.journal != nil && j.journal.path == path
	}
	if path == directory {
		return j.dirs[path] != nil
	}
	if path == pending {
		for _, f := range j.files {
			if f.path == path {
				return true
			}
		}
	}
	return false
}
