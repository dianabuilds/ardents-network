package forwarding

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"
)

// readSelections follows only the exact predecessor path/commitment. It retains
// every sealed generation independently; no fresh Release or retirement right
// follows from a historical selection or a complete-looking journal.
func (c *completedInitial) readSelections(selected generationSelection) error {
	f, current := c.files, selected
	c.selections = map[string]generationSelection{}
	seen := map[string]bool{}
	for len(c.selectionOrder) < attemptLimit {
		b := current.binding
		if seen[b.Generation] {
			return ErrBinding
		}
		seen[b.Generation] = true
		files, err := c.readGeneration(current)
		if err != nil {
			return err
		}
		plan, err := validateSelectedGeneration(b, current.raw, files)
		if err != nil {
			return err
		}
		if len(c.selectionOrder) == 0 {
			c.plan = plan
		}
		c.selectionOrder = append(c.selectionOrder, current)
		c.selections[digest(current.raw)] = current
		if current.replacement == nil {
			if current.binding.Generation != selected.initialGeneration() {
				return ErrBinding
			}
			c.initial = current
			break
		}
		path := filepath.Join(b.Request.InstallationRoot, "generations", current.replacement.PredecessorGeneration, "binding.json")
		raw, err := f.read(path, 32<<10, 0444, 0, 0)
		if err != nil {
			return err
		}
		previous, err := decodeSelection(raw)
		if err != nil || current.matchPredecessor(previous) != nil {
			return errors.Join(ErrBinding, err)
		}
		current = previous
	}
	if len(c.initial.raw) == 0 {
		return ErrBinding
	}
	for _, s := range c.selectionOrder {
		f.generationNames = append(f.generationNames, s.binding.Generation)
		if s.replacement != nil {
			f.replacementJournals = append(f.replacementJournals, replacementJournal(s.binding.Request.InstallationRoot, s.binding.Generation))
		}
	}
	slices.Sort(f.generationNames)
	f.generationRoot = filepath.Join(selected.binding.Request.InstallationRoot, "generations")
	return nil
}

func (c *completedInitial) readGeneration(selected generationSelection) (map[string][]byte, error) {
	f, b := c.files, selected.binding
	sealed := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	if err := f.directory(sealed, 0555, 0, 0); err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for _, name := range forwardingGenerationNames() {
		maximum, mode := int64(1<<20), os.FileMode(0444)
		if name == programName {
			maximum, mode = 64<<20, 0555
		} else if name == "binding.json" {
			maximum = 32 << 10
		} else if name != "root.json" && name != "timestamp.json" && name != "snapshot.json" && name != "targets.json" {
			maximum = 16 << 10
		}
		path := filepath.Join(sealed, name)
		if original := f.files[path]; original != nil {
			files[name] = original.body
		} else {
			raw, err := f.read(path, maximum, mode, 0, 0)
			if err != nil {
				return nil, err
			}
			files[name] = raw
		}
	}
	if f.extraInventories == nil {
		f.extraInventories = map[string]int{}
	}
	f.extraInventories[sealed] = 12
	return files, nil
}

func (c *completedInitial) initialHashes(bindingPath string) map[string]string {
	f, b := c.files, c.initial.binding
	sealed := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	hashes := map[string]string{}
	for _, name := range forwardingGenerationNames() {
		path := filepath.Join(sealed, name)
		hashes[path] = digest(f.files[path].body)
	}
	for _, name := range []string{"certificate.pem", "private-key.pem"} {
		path := filepath.Join(b.Request.CredentialDirectory, name)
		hashes[path] = digest(f.files[path].body)
	}
	hashes[bindingPath] = digest(c.initial.raw)
	hashes["/etc/systemd/system/ardents-forwarding.service"] = b.UnitSHA256
	hashes[f.pending] = digest([]byte("ardents-forwarding-initial-pending-v1\n" + digest(c.initial.raw) + "\n"))
	return hashes
}

func (c *completedInitial) readReplacementHistory() error {
	f := c.files
	for _, selected := range c.selectionOrder {
		r := selected.replacement
		if r == nil {
			continue
		}
		previous, found := c.selections[r.PredecessorBindingSHA256]
		if !found {
			return ErrBinding
		}
		b := selected.binding
		journal := replacementJournal(b.Request.InstallationRoot, b.Generation)
		file, err := f.retain(journal, 4<<20, 0600, 0, 0)
		if err != nil {
			return err
		}
		if f.interrupted == nil || f.interrupted.cut.Boundary.Journal != journal {
			if err := syscall.Flock(int(file.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				return err
			}
		} else if err := f.interrupted.check(); err != nil {
			return err
		}
		raw, err := f.fill(file, 4<<20)
		if err != nil {
			return err
		}
		hashes := map[string]string{}
		sealed := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
		for _, name := range forwardingGenerationNames() {
			hashes[name] = digest(f.files[filepath.Join(sealed, name)].body)
		}
		rows, err := decodeTransitionPrefix(raw, "replacement")
		if err != nil {
			return err
		}
		var before, after []byte
		var proof *replacementNativeProof
		activePreReload := f.interrupted != nil && f.interrupted.cut.Boundary.Operation == "replacement" && f.interrupted.cut.Boundary.Journal == journal && (f.interrupted.cut.Boundary.Records == 57 || f.interrupted.cut.Boundary.Records == 58)
		if rows[0].Phase == "forwarding-replacement-v2" {
			beforePath, afterPath := replacementNativePaths(journal)
			before, err = f.read(beforePath, 64<<10, 0600, 0, 0)
			if err != nil {
				return err
			}
			if !activePreReload {
				after, err = f.read(afterPath, 64<<10, 0600, 0, 0)
				if err != nil {
					return err
				}
				proof = &replacementNativeProof{f.files[beforePath], f.files[afterPath]}
			} else {
				f.replacementJournals = append(f.replacementJournals, beforePath)
			}
		}
		logical, err := c.readRecoveredReplacement(file, selected, previous, hashes, before, after)
		if err != nil {
			return err
		}
		var owned map[string]initialRecord
		if activePreReload {
			if c.replacementPreReload(journal) == nil {
				return ErrBinding
			}
			owned, err = decodeReplacementPreReloadRecords(logical, selected, previous, hashes, before)
		} else {
			owned, err = decodeReplacementContextRecords(logical, selected, previous, hashes, before, after)
		}
		if err != nil {
			return err
		}
		retirement := filepath.Join(attemptNumber(r.PredecessorAttempt).retirement(b.Request.InstallationRoot), "retirement-result.json")
		if f.files[retirement] == nil {
			if _, err := f.read(retirement, 64<<10, 0600, 0, 0); err != nil {
				return err
			}
		}
		if digest(f.files[retirement].body) != r.RetirementSHA256 {
			return ErrBinding
		}
		for path, record := range owned {
			if path == replacementPending(b.Request.InstallationRoot) {
				if current := f.files[path]; current != nil && selected.binding.Generation == c.binding.Generation {
					native, ok := infoSys(current.info)
					if !ok || uint64(native.Dev) != record.Device || native.Ino != record.Inode {
						return ErrBinding
					}
				}
				continue
			}
			var info os.FileInfo
			if file := f.files[path]; file != nil {
				info = file.info
			} else if directory := f.dirs[path]; directory != nil {
				info = directory.info
			}
			if info == nil || !nodeAccess(info, os.FileMode(record.Mode), record.UID, record.GID, info.IsDir()) {
				return ErrBinding
			}
			native := info.Sys().(*syscall.Stat_t)
			if uint64(native.Dev) != record.Device || native.Ino != record.Inode {
				return ErrBinding
			}
		}
		if proof != nil {
			if c.reloads == nil {
				c.reloads = map[string]*replacementNativeProof{}
			}
			c.reloads[b.Generation] = proof
			f.replacementJournals = append(f.replacementJournals, proof.before.path, proof.after.path)
		}
	}
	return nil
}
