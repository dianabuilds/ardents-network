package forwarding

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The initial and every start/retirement lease remain independently held. No
// missing predecessor, partial suffix or recycled InvocationID can be skipped.
func (s *started) readChain(bindingPath string) error {
	c := s.completed.original
	root := c.binding.Request.InstallationRoot
	entries, err := historyDirectoryEntries(c.files.dirs[root])
	if err != nil {
		return err
	}
	var names []string
	for _, entry := range entries {
		if entry.Name() == "forwarding-startup" || strings.HasPrefix(entry.Name(), "forwarding-invocation-") {
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return ErrBinding
			}
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return os.ErrNotExist
	}
	latest, err := attemptDirectories(names)
	if err != nil {
		return err
	}
	bind, initial := digest(c.files.files[bindingPath].body), digest(c.files.journal.body)
	predecessor := ""
	seen := map[[16]byte]bool{}
	for n := attemptNumber(0); n <= latest; n++ {
		h, err := readStartSnapshotAt(s.ctx, c.binding, n, bind, initial, predecessor)
		if h != nil {
			s.starts = append(s.starts, h)
		}
		if err != nil {
			return err
		}
		if seen[h.id] {
			return ErrBinding
		}
		seen[h.id] = true
		s.snapshot = h
		needsRetirement := n < latest
		if !needsRetirement {
			for _, path := range []string{n.retirement(root), n.retirementJournal(root)} {
				if _, err := os.Lstat(path); err == nil {
					needsRetirement = true
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
		}
		if needsRetirement {
			r, err := readRetirementSnapshot(s.ctx, c.binding, n, bind, h.sha256, h.archive, h.id, retirementCommitted)
			if r != nil {
				s.retirements = append(s.retirements, r)
			}
			if err != nil {
				return err
			}
			predecessor = r.sha256
			if n == latest {
				s.latestRetirement = r
			}
		}
	}
	return s.checkNamespace(nil, nil)
}

func historyDirectoryEntries(directory *inputDirectory) ([]os.DirEntry, error) {
	if directory == nil || directory.file == nil {
		return nil, ErrBinding
	}
	if _, err := directory.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	entries, err := directory.file.ReadDir(historyDirectoryLimit)
	if err != nil && err != io.EOF || len(entries) >= historyDirectoryLimit {
		return nil, errors.Join(ErrBinding, err)
	}
	return entries, nil
}

func (s *started) checkChain(start *startStorage, retirement *retirementStorage) error {
	if s.snapshot == nil || len(s.starts) == 0 {
		return ErrBinding
	}
	if err := s.checkNamespace(start, retirement); err != nil {
		return err
	}
	for _, h := range s.starts {
		if err := h.check(); err != nil {
			return err
		}
	}
	for _, h := range s.retirements {
		if err := h.check(); err != nil {
			return err
		}
	}
	return s.checkNamespace(start, retirement)
}

// A root producer may include ONLY its own actual newborn storage at the next
// start or latest stop boundary. All older files retain their separate pins.
func (s *started) checkNamespace(start *startStorage, retirement *retirementStorage) error {
	if s.snapshot == nil || s.completed.original == nil || start != nil && retirement != nil {
		return ErrBinding
	}
	c := s.completed.original
	root, latest := c.binding.Request.InstallationRoot, s.snapshot.number
	if latest >= attemptLimit || len(s.starts) != int(latest)+1 {
		return ErrBinding
	}
	if start != nil {
		if s.latestRetirement == nil || latest+1 >= attemptLimit || start.number != latest+1 || start.path != (latest+1).directory(root) {
			return ErrBinding
		}
		if err := start.check(); err != nil {
			return err
		}
	}
	if retirement != nil {
		if s.latestRetirement != nil || retirement.number != latest || retirement.path != latest.retirement(root) {
			return ErrBinding
		}
		if err := retirement.check(); err != nil {
			return err
		}
	}
	expected := map[string]bool{}
	for n := attemptNumber(0); n <= latest; n++ {
		expected[filepath.Base(n.directory(root))] = true
		if n < latest || s.latestRetirement != nil {
			expected[filepath.Base(n.retirement(root))] = true
		}
	}
	if start != nil && start.dir != nil {
		expected[filepath.Base(start.path)] = true
	}
	if retirement != nil && retirement.dir != nil {
		expected[filepath.Base(retirement.path)] = true
	}
	entries, err := historyDirectoryEntries(c.files.dirs[root])
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != "forwarding-startup" && !strings.HasPrefix(name, "forwarding-invocation-") && !strings.HasPrefix(name, "forwarding-retirement-") {
			continue
		}
		if !expected[name] || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return ErrBinding
		}
		delete(expected, name)
	}
	if len(expected) != 0 {
		return ErrBinding
	}
	for n := attemptNumber(0); n < attemptLimit; n++ {
		startPresent := n <= latest || start != nil && n == start.number
		retirePresent := n < latest || n == latest && s.latestRetirement != nil || retirement != nil && n == retirement.number
		for _, item := range []struct {
			path    string
			present bool
		}{{n.journal(root), startPresent}, {n.retirementJournal(root), retirePresent}} {
			_, err := os.Lstat(item.path)
			if item.present {
				if err != nil {
					return errors.Join(ErrBinding, err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return errors.Join(ErrBinding, err)
			}
		}
	}
	return s.ctx.Err()
}
