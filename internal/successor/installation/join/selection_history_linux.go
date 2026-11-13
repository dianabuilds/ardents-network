package join

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"
)

// These are retained byte/native histories, never live domain or Release rights.
func (c *completedInitial) readSelections(current generationSelection) error {
	c.selections = map[string]generationSelection{}
	c.selectionOrder = nil
	seen := map[string]bool{}
	next := current
	for {
		b := next.binding
		if len(c.selectionOrder) >= attemptLimit || seen[b.Generation] {
			return ErrBinding
		}
		seen[b.Generation] = true
		sealed := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
		if err := c.directory(sealed, 0555, 0, 0); err != nil {
			return err
		}
		image := map[string][]byte{}
		for _, name := range joinGenerationNames() {
			maximum, mode := int64(1<<20), os.FileMode(0444)
			if name == programName {
				maximum, mode = 64<<20, 0555
			}
			f, err := c.read(filepath.Join(sealed, name), maximum, mode, 0, 0)
			if err != nil {
				return err
			}
			image[name] = f.body
		}
		if _, err := validateInitialGeneration(b, next.raw, image); err != nil {
			return err
		}
		c.selections[digest(next.raw)] = next
		c.selectionOrder = append(c.selectionOrder, next)
		if next.replacement == nil {
			c.initialSelection = next
			break
		}
		previousRawPath := filepath.Join(b.Request.InstallationRoot, "generations", next.replacement.PredecessorGeneration, "binding.json")
		// Parent bytes are read from their original protected file, then re-opened
		// into the full independent custody on the next loop. No path grants authority.
		parent, err := c.parents.parents(previousRawPath)
		if err != nil {
			return err
		}
		f, err := openInputAt(parent, filepath.Base(previousRawPath), false)
		if err != nil {
			return err
		}
		raw, err := readSelectionFile(f)
		err = errors.Join(err, f.Close())
		if err != nil {
			return err
		}
		previous, err := decodeSelection(raw)
		if err != nil {
			return err
		}
		if err = next.matchPredecessor(previous); err != nil {
			return err
		}
		next = previous
	}
	slices.Reverse(c.selectionOrder)
	return nil
}

func readSelectionFile(f *os.File) ([]byte, error) {
	info, err := f.Stat()
	if err != nil || !nodeAccess(info, 0444, 0, 0, false) || info.Size() < 1 || info.Size() > 32<<10 {
		return nil, errors.Join(ErrBinding, err)
	}
	raw := make([]byte, info.Size())
	n, err := f.ReadAt(raw, 0)
	if err != nil || n != len(raw) {
		return nil, errors.Join(ErrBinding, err)
	}
	after, err := f.Stat()
	if err != nil || !sameInputFile(info, after) {
		return nil, errors.Join(ErrBinding, err)
	}
	return raw, nil
}

func replacementJournal(root, generation string) string {
	return root + ".join-replacement-" + generation + "-journal"
}
func replacementNativePaths(journal string) (string, string) {
	return journal + ".native-before", journal + ".native-after"
}
func replacementPending(root string) string { return root + ".join-replacement-pending" }
func replacementPendingBytes(raw []byte) []byte {
	return []byte("ardents-join-replacement-pending-v1\n" + digest(raw) + "\n")
}

func (c *completedInitial) readReplacements() error {
	c.replacementRetirements = map[string]initialRecord{}
	for i, selected := range c.selectionOrder {
		if i == 0 {
			continue
		}
		old := c.selectionOrder[i-1]
		b := selected.binding
		oldDir := filepath.Join(old.binding.Request.InstallationRoot, "generations", old.binding.Generation)
		nextDir := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
		if err := validateForwardImage(c.files[filepath.Join(oldDir, "infrastructure-join.json")].body, c.files[filepath.Join(nextDir, "infrastructure-join.json")].body, c.files[filepath.Join(oldDir, "join-plan.json")].body, c.files[filepath.Join(nextDir, "join-plan.json")].body, b.ProgramSHA256 != old.binding.ProgramSHA256); err != nil {
			return err
		}
		journal := replacementJournal(b.Request.InstallationRoot, b.Generation)
		j, err := c.read(journal, 4<<20, 0600, 0, 0)
		if err != nil {
			return err
		}
		if err = syscall.Flock(int(j.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			return err
		}
		if err = observeInitialFile(j); err != nil {
			return err
		}
		beforePath, afterPath := replacementNativePaths(journal)
		before, err := c.read(beforePath, 64<<10, 0600, 0, 0)
		if err != nil {
			return err
		}
		after, err := c.read(afterPath, 64<<10, 0600, 0, 0)
		if err != nil {
			return err
		}
		hashes := map[string]string{}
		sealed := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
		for _, name := range joinGenerationNames() {
			hashes[filepath.Join(sealed, name)] = digest(c.files[filepath.Join(sealed, name)].body)
		}
		records, err := decodeReplacementRecords(j.body, selected, old, hashes, before.body, after.body)
		if err != nil {
			return err
		}
		for path, r := range records {
			if path == filepath.Join(attemptNumber(selected.replacement.PredecessorAttempt).retirement(b.Request.InstallationRoot), "retirement-result.json") {
				c.replacementRetirements[path] = r
				continue
			}
			if path == replacementPending(b.Request.InstallationRoot) {
				continue
			}
			var info os.FileInfo
			if f := c.files[path]; f != nil {
				info = f.info
			} else if d := c.directoryAt(path); d != nil {
				info = d.info
			}
			st, ok := infoSys(info)
			if !ok || uint64(st.Dev) != r.Device || st.Ino != r.Inode || !nodeAccess(info, os.FileMode(r.Mode), r.UID, r.GID, info.IsDir()) {
				return ErrBinding
			}
		}
		c.replacementPaths = append(c.replacementPaths, journal, beforePath, afterPath)
	}
	return nil
}

func (c *completedInitial) initialBindingBytes() []byte {
	if len(c.initialSelection.raw) != 0 {
		return c.initialSelection.raw
	}
	return c.files[filepath.Join(c.binding.Request.InstallationRoot, "binding.json")].body
}

func (c *completedInitial) selectionForAttempt(n attemptNumber) generationSelection {
	selected := c.initialSelection
	for _, s := range c.selectionOrder {
		if s.replacement != nil && uint32(n) > s.replacement.PredecessorAttempt {
			selected = s
		}
	}
	return selected
}

func (c *completedInitial) checkReplacementLineage() error {
	for _, s := range c.selectionOrder {
		r := s.replacement
		if r == nil {
			continue
		}
		h := c.invocations
		if h == nil || int(r.PredecessorAttempt) >= len(h.retirements) {
			return ErrBinding
		}
		retired := h.retirements[r.PredecessorAttempt]
		path := filepath.Join(retired.path, "retirement-result.json")
		record, exists := c.replacementRetirements[path]
		file := retired.files[path]
		if !exists || file == nil {
			return ErrBinding
		}
		native, ok := infoSys(file.info)
		if !ok || record.Device != uint64(native.Dev) || record.Inode != native.Ino || record.Mode != uint32(file.info.Mode().Perm()) || record.UID != native.Uid || record.GID != native.Gid {
			return ErrBinding
		}
		beforePath, _ := replacementNativePaths(replacementJournal(c.binding.Request.InstallationRoot, s.binding.Generation))
		before, err := decodeNativeContext(c.files[beforePath].body)
		if err != nil || before.Manager != retired.result.Manager {
			return ErrBinding
		}

		if retired.sha256 != r.RetirementSHA256 || retired.bindingSHA != r.PredecessorBindingSHA256 || retired.binding.Generation != r.PredecessorGeneration {
			return ErrBinding
		}
	}
	return nil
}

// Native guard namespace inventory includes every unknown sibling, including
// an empty journal born before a pending marker. A known name still needs its
// independently verified typed transcript and original native custody.
func replacementSiblings(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Dir(root))
	if err != nil || len(entries) > 4096 {
		return nil, errors.Join(ErrBinding, err)
	}
	prefix := filepath.Base(root) + ".join-replacement-"
	var paths []string
	for _, e := range entries {
		if len(e.Name()) >= len(prefix) && e.Name()[:len(prefix)] == prefix {
			paths = append(paths, filepath.Join(filepath.Dir(root), e.Name()))
		}
	}
	return paths, nil
}
