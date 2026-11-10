package forwarding

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
)

var errCompletedClosed = errors.New("forwarding installation: completed observation closed")

// completedFiles owns fresh read-only descriptors; it never borrows Inputs or
// the creator's writer, native maps, pending inode or completion result. Root's
// separate completed reader assigns its exclusive journal observation and keeps
// it through all other closes. Runtime constructs its own fresh instance without
// any journal; this physical read machinery grants neither caller a live right.
type completedFiles struct {
	ctx                   context.Context
	mu, ioMu              sync.Mutex
	uses                  sync.WaitGroup
	once                  sync.Once
	closed                bool
	failure               error
	files                 map[string]*initialFile
	fileOrder             []*initialFile
	executable            *completedExecutableView
	dirs                  map[string]*inputDirectory
	dirOrder              []*inputDirectory
	journal               *initialFile
	clock                 string
	pending               string
	sealed                string
	credential            string
	absentPaths           []string
	extraInventories      map[string]int
	generationRoot        string
	generationNames       []string
	replacementJournals   []string
	recovered             *recoveredInitial
	recoveryContext       *recoveryContext
	recoveredReplacements []*recoveredReplacement
	interrupted           *recoveryOriginal
	recoveryPublishing    *recoveryPublication
	recoveryReadback      *recoveryPublication
	invocations           *invocationChain
	// Negative native controls only, after actual I/O or before actual Close.
	observe func(string, *os.File)
}

func (s *completedFiles) parents(path string) (*inputDirectory, error) {
	if !validPath(path) {
		return nil, ErrInput
	}
	parts := append([]string{""}, strings.Split(strings.TrimPrefix(filepath.Dir(path), "/"), "/")...)
	current := "/"
	var parent *inputDirectory
	for i, name := range parts {
		if i > 0 && name == "" {
			continue
		}
		if i > 0 {
			current = filepath.Join(current, name)
		}
		if original := s.dirs[current]; original != nil {
			if !trustedInputDirectory(original.info) {
				return nil, ErrBinding
			}
			parent = original
			continue
		}
		var file *os.File
		var err error
		if parent == nil {
			file, err = os.OpenFile("/", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
		} else {
			file, err = openInputAt(parent, name, true)
		}
		if err != nil {
			return nil, err
		}
		original := &inputDirectory{path: current, name: name, file: file, parent: parent}
		s.dirs[current] = original
		s.dirOrder = append(s.dirOrder, original)
		original.info, err = file.Stat()
		if err != nil || !trustedInputDirectory(original.info) {
			return nil, errors.Join(ErrBinding, err)
		}
		parent = original
	}
	return parent, s.local()
}

func (s *completedFiles) directory(path string, mode os.FileMode, uid, gid uint32) error {
	parent, err := s.parents(path)
	if err != nil {
		return err
	}
	if original := s.dirs[path]; original != nil {
		if !nodeAccess(original.info, mode, uid, gid, true) {
			return ErrBinding
		}
		return s.local()
	}
	file, err := openInputAt(parent, filepath.Base(path), true)
	if err != nil {
		return err
	}
	original := &inputDirectory{path: path, name: filepath.Base(path), file: file, parent: parent}
	s.dirs[path] = original
	s.dirOrder = append(s.dirOrder, original)
	original.info, err = file.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(file.Fd()), &fs)
	if err != nil || fsErr != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 || !nodeAccess(original.info, mode, uid, gid, true) {
		return errors.Join(ErrBinding, err, fsErr)
	}
	return s.local()
}

func (s *completedFiles) retain(path string, maximum int64, mode os.FileMode, uid, gid uint32) (*initialFile, error) {
	if s.files[path] != nil {
		return nil, ErrInput
	}
	parent, err := s.parents(path)
	if err != nil {
		return nil, err
	}
	return s.retainAt(path, parent, maximum, mode, uid, gid)
}

// retainAt opens only beneath this reader's already retained exact directory.
// The caller must independently admit that directory's specific access policy;
// ordinary retain keeps its stricter root:root ancestor walk.
func (s *completedFiles) retainAt(path string, parent *inputDirectory, maximum int64, mode os.FileMode, uid, gid uint32) (*initialFile, error) {
	if !validPath(path) || s.files[path] != nil || parent == nil || parent.file == nil || parent.info == nil || parent.path != filepath.Dir(path) || s.dirs[parent.path] != parent {
		return nil, ErrInput
	}
	file, err := openInputAt(parent, filepath.Base(path), false)
	if err != nil {
		return nil, err
	}
	original := &initialFile{path: path, file: file}
	s.files[path] = original
	s.fileOrder = append(s.fileOrder, original)
	original.info, err = file.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(file.Fd()), &fs)
	if err != nil || fsErr != nil || fs.Type != 0xef53 || !nodeAccess(original.info, mode, uid, gid, false) || original.info.Size() < 1 || original.info.Size() > maximum {
		return nil, errors.Join(ErrBinding, err, fsErr)
	}
	return original, s.local()
}

func (s *completedFiles) read(path string, maximum int64, mode os.FileMode, uid, gid uint32) ([]byte, error) {
	original, err := s.retain(path, maximum, mode, uid, gid)
	if err != nil {
		return nil, err
	}
	return s.fill(original, maximum)
}

func (s *completedFiles) fill(original *initialFile, maximum int64) ([]byte, error) {
	if err := s.local(); err != nil {
		return nil, err
	}
	if original == nil || original.file == nil || original.info == nil || original.info.Size() < 0 || original.info.Size() > maximum {
		return nil, ErrBinding
	}
	// Native custody already bounded this exact original size. Allocate once
	// rather than geometrically growing and copying the complete program image.
	// A separate read still proves exact EOF, including a file grown after Stat.
	raw := make([]byte, original.info.Size())
	original.body = raw
	for offset := 0; offset < len(raw); {
		if err := s.local(); err != nil {
			return nil, err
		}
		end := offset + min(64<<10, len(raw)-offset)
		if _, err := io.ReadFull(original.file, raw[offset:end]); err != nil {
			return nil, errors.Join(ErrBinding, err)
		}
		offset = end
		if s.observe != nil {
			s.observe("fill-chunk", original.file)
		}
		// Initial acquisition needs the same bounded physical reads as later
		// comparisons, without growing or copying the retained byte image.
		runtime.Gosched()
	}
	if err := s.local(); err != nil {
		return nil, err
	}
	var extra [1]byte
	n, err := original.file.Read(extra[:])
	if n != 0 || err != io.EOF {
		return nil, errors.Join(ErrBinding, err)
	}
	if s.observe != nil {
		s.observe("read", original.file)
	}
	return raw, s.local()
}

func (s *completedFiles) absent(path string) error {
	parent := s.dirs[filepath.Dir(path)]
	if parent == nil {
		return ErrBinding
	}
	file, err := openInputAt(parent, filepath.Base(path), false)
	if file != nil {
		return errors.Join(ErrRepairRequired, file.Close())
	}
	if !errors.Is(err, syscall.ENOENT) {
		return errors.Join(ErrBinding, err)
	}
	return s.local()
}

func (s *completedFiles) links() error {
	for _, original := range s.dirOrder {
		current, err := original.file.Stat()
		named, nameErr := os.Lstat(original.path)
		before := original.info
		if before == nil {
			return errors.Join(ErrBinding, err, nameErr)
		}
		native := before.Sys().(*syscall.Stat_t)
		if err != nil || nameErr != nil || !os.SameFile(before, current) || !os.SameFile(before, named) || !nodeAccess(current, before.Mode().Perm(), native.Uid, native.Gid, true) || !nodeAccess(named, before.Mode().Perm(), native.Uid, native.Gid, true) {
			return errors.Join(ErrBinding, err, nameErr)
		}
		if original.parent != nil {
			named, err := statCompletedName(original.parent, original.name, true)
			if err != nil {
				return err
			}
			if !sameCompletedDirectory(before, named) {
				return ErrBinding
			}
		}
	}
	for _, original := range s.fileOrder {
		info, err := original.file.Stat()
		if err != nil || !sameInputFile(original.info, info) {
			return errors.Join(ErrBinding, err)
		}
		parent := s.dirs[filepath.Dir(original.path)]
		named, err := statCompletedName(parent, filepath.Base(original.path), false)
		if err != nil {
			return err
		}
		if !sameCompletedFile(original.info, named) {
			return ErrBinding
		}
	}
	return s.local()
}

func (s *completedFiles) paths() error {
	if err := s.links(); err != nil {
		return err
	}
	if s.recovered == nil || s.recovered.originalPending == nil {
		if err := s.absent(s.pending); err != nil {
			return err
		}
	}
	for _, path := range s.absentPaths {
		if err := s.absent(path); err != nil {
			return err
		}
	}
	if err := checkCompletedClock(s.dirs[filepath.Dir(s.clock)], s.clock); err != nil {
		return err
	}
	return s.inventory()
}

func (s *completedFiles) inventory() error {
	if s.generationRoot != "" {
		root := filepath.Dir(s.generationRoot)
		parent := s.dirs[filepath.Dir(root)]
		if err := checkCompletedJournalInventory(root, parent, s.replacementJournals, s); err != nil {
			return err
		}
	}
	if s.generationRoot != "" {
		dir := s.dirs[s.generationRoot]
		if dir == nil {
			return ErrBinding
		}
		if _, err := dir.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		entries, err := dir.file.ReadDir(len(s.generationNames) + 1)
		if err != nil && err != io.EOF || len(entries) != len(s.generationNames) {
			return errors.Join(ErrBinding, err)
		}
		names := make([]string, len(entries))
		for n, entry := range entries {
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return ErrBinding
			}
			names[n] = entry.Name()
		}
		slices.Sort(names)
		if !slices.Equal(names, s.generationNames) {
			return ErrBinding
		}
	}
	inventories := map[string]int{s.sealed: 12, s.credential: 2}
	for path, count := range s.extraInventories {
		inventories[path] = count
	}
	for path, count := range inventories {
		dir := s.dirs[path]
		if dir == nil {
			return ErrBinding
		}
		if _, err := dir.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		entries, err := dir.file.ReadDir(count + 1)
		if err != nil && err != io.EOF || len(entries) != count {
			return errors.Join(ErrBinding, err)
		}
		for _, entry := range entries {
			if s.files[filepath.Join(path, entry.Name())] == nil || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return ErrBinding
			}
		}
	}
	return nil
}

// A journal's exclusive birth precedes its pending file. Every ordinary reader
// must refuse an unknown sibling, including an empty/torn journal at that cut.
// Enumerating names supplies no contents, original inode or completion proof.
func checkCompletedJournalInventory(root string, parent *inputDirectory, journals []string, files *completedFiles) error {
	if parent == nil || parent.file == nil || parent.path != filepath.Dir(root) {
		return ErrBinding
	}
	if _, err := parent.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	entries, err := parent.file.ReadDir(4097)
	if err != nil && err != io.EOF || len(entries) > 4096 {
		return errors.Join(ErrBinding, err)
	}
	expected := map[string]bool{}
	for _, path := range journals {
		if filepath.Dir(path) != parent.path || expected[filepath.Base(path)] {
			return ErrBinding
		}
		expected[filepath.Base(path)] = true
	}
	prefix := filepath.Base(root) + ".forwarding-replacement-"
	recoveryNames := map[string]bool{}
	addRecovery := func(paths []string) error {
		for _, path := range paths {
			if filepath.Dir(path) != parent.path || recoveryNames[filepath.Base(path)] {
				return ErrBinding
			}
			recoveryNames[filepath.Base(path)] = true
		}
		return nil
	}
	if files != nil && files.recovered != nil {
		if err := files.recovered.check(); err != nil {
			return err
		}
		if err := addRecovery(files.recovered.paths); err != nil {
			return err
		}
	}
	if files != nil {
		if chain := files.invocations; chain != nil {
			for _, terminal := range chain.terminals {
				if err := terminal.check(); err != nil {
					return err
				}
				if err := addRecovery(terminal.paths); err != nil {
					return err
				}
			}
			for _, retirement := range chain.retirements {
				if proof := retirement.recovery; proof != nil {
					if err := proof.check(retirement); err != nil {
						return err
					}
					if err := addRecovery(proof.paths); err != nil {
						return err
					}
				}
			}
		}
		for _, proof := range files.recoveredReplacements {
			if err := proof.check(); err != nil {
				return err
			}
			if err := addRecovery(proof.paths); err != nil {
				return err
			}
		}
		publication := files.recoveryPublishing
		if publication != nil && publication.births != nil {
			original := files.interrupted
			if original == nil || original.cut.Boundary.Journal != publication.completion.Journal || publication.births.journal == nil {
				return ErrBinding
			}
			if err := errors.Join(original.check(), publication.births.check()); err != nil {
				return err
			}
			control, result, pending := recoveryPaths(publication.completion.Journal)
			if publication.births.journal.path != control {
				return ErrBinding
			}
			paths := []string{control}
			var reloadBefore, reloadAfter string
			if reload := publication.replacementReload; reload != nil {
				if reload.history == nil || reload.history.completed.original == nil || reload.history.completed.original.files != files || reload.original != original || reload.beforeReload == nil || reload.beforeReload.owner != files || reload.beforeReload.original != original {
					return ErrBinding
				}
				var err error
				reloadBefore, reloadAfter, err = recoveryReloadProofPaths(publication.completion.Journal, "replacement")
				if err != nil {
					return err
				}
			}
			for _, file := range publication.births.files {
				if reloadAfter != "" && file.path == reloadAfter {
					name := filepath.Base(file.path)
					if filepath.Dir(file.path) != parent.path || expected[name] || !publication.births.allowed[file.path] {
						return ErrBinding
					}
					expected[name] = true
					continue
				}
				if file.path != result && file.path != pending && (reloadBefore == "" || file.path != reloadBefore) {
					return ErrBinding
				}
				paths = append(paths, file.path)
			}
			if err := addRecovery(paths); err != nil {
				return err
			}
		}
	}
	for _, entry := range entries {
		family := strings.TrimPrefix(entry.Name(), filepath.Base(root)+".forwarding-")
		if family != entry.Name() && (strings.Contains(family, ".original-cut-") || strings.Contains(family, ".recovery-")) {
			// Any cut/recovery sidecar forces its independent complete recovered
			// validation, even when the original operation's blocker is absent.
			// An expected replacement name cannot waive this admission boundary.
			if !recoveryNames[entry.Name()] || !entry.Type().IsRegular() {
				return ErrBinding
			}
			delete(recoveryNames, entry.Name())
			continue
		}
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		if !expected[entry.Name()] || !entry.Type().IsRegular() {
			return ErrBinding
		}
		delete(expected, entry.Name())
	}
	if len(expected) != 0 || len(recoveryNames) != 0 {
		return ErrBinding
	}
	return nil
}
