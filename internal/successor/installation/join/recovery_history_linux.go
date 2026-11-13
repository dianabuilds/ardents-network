package join

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// Recovery is a separate completion over an immutable original prefix. No row
// is appended to, repaired in, or substituted for the original journal.
type recoveryCompletion struct {
	Schema            string `json:"schema"`
	Journal           string `json:"journal"`
	Device            uint64 `json:"device"`
	Inode             uint64 `json:"inode"`
	PrefixSHA256      string `json:"prefix_sha256"`
	OriginalCutSHA256 string `json:"original_cut_sha256"`
	BindingSHA256     string `json:"binding_sha256"`
	Outcome           string `json:"outcome"`
}

func recoveryPaths(journal string) (string, string, string) {
	return journal + ".recovery-journal", journal + ".recovery-result", journal + ".recovery-pending"
}
func recoveryPendingBytes(raw []byte) []byte {
	return []byte("ardents-join-recovery-pending-v1\n" + digest(raw) + "\n")
}
func decodeRecoveryCompletion(raw, prefix, cut []byte, b transitionCutResult) (recoveryCompletion, error) {
	var r recoveryCompletion
	if len(raw) == 0 || len(raw) > 8192 || json.Unmarshal(raw, &r) != nil {
		return r, ErrBinding
	}
	canonical, _ := json.Marshal(r)
	wanted := recoveryCompletion{"ardents-join-recovery-completion-v1", b.Boundary.Journal, b.Boundary.Device, b.Boundary.Inode, digest(prefix), digest(cut), b.Boundary.BindingSHA256, "selected-stopped"}
	if !bytes.Equal(raw, append(canonical, '\n')) || r != wanted || b.Boundary.Operation != "replacement" || b.Boundary.Records != 62 {
		return r, ErrBinding
	}
	return r, nil
}

// Called under this completed operation's original journal lease. Cut storage
// has its own exclusion and descriptors; detached result presence grants nothing.
func (c *completedInitial) readReplacementRecovery(j *initialFile, selected generationSelection, pending *initialFile) error {
	controlPath, resultPath, cutPending := transitionCutPaths(j.path)
	if _, err := os.Lstat(controlPath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	control, err := c.read(controlPath, 64<<10, 0600, 0, 0)
	if err != nil {
		return err
	}
	if err = syscall.Flock(int(control.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return err
	}
	result, err := c.read(resultPath, 256<<10, 0600, 0, 0)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(cutPending); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(ErrBinding, err)
	}
	c.absent = append(c.absent, cutPending)
	cut, err := decodeTransitionCut(control.body, result.body, j.body, control.info, result.info, j.info)
	if err != nil || cut.Boundary.Operation != "replacement" || cut.Boundary.Records != 62 || !bytes.Equal(cut.Boundary.Binding, selected.raw) || cut.Origin.ProgramSHA256 != selected.binding.ProgramSHA256 {
		return errors.Join(ErrBinding, err)
	}
	beforePath, afterPath := replacementNativePaths(j.path)
	if !bytes.Equal(c.files[beforePath].body, cut.Boundary.OriginalContext) || !bytes.Equal(c.files[afterPath].body, cut.Boundary.NativeContext) {
		return ErrBinding
	}
	c.replacementPaths = append(c.replacementPaths, controlPath, resultPath)
	suffix, suffixResult, suffixPending := recoveryPaths(j.path)
	if pending != nil && pending.path == replacementPending(selected.binding.Request.InstallationRoot) {
		// The sole repair opener may inspect a complete original while its blocker
		// remains. It cannot admit Start or bypass a partially born recovery suffix.
		if _, err = os.Lstat(suffix); errors.Is(err, os.ErrNotExist) {
			for _, path := range []string{suffixResult, suffixPending} {
				if _, err = os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					return errors.Join(ErrBinding, err)
				}
			}
			c.recoveryCut = &cut
			c.recoveryOriginal = result
			return nil
		} else if err != nil {
			return err
		}
	}
	journal, err := c.read(suffix, 64<<10, 0600, 0, 0)
	if err != nil {
		return err
	}
	if err = syscall.Flock(int(journal.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return err
	}
	complete, err := c.read(suffixResult, 8192, 0600, 0, 0)
	if err != nil {
		return err
	}
	if _, err = decodeRecoveryCompletion(complete.body, j.body, result.body, cut); err != nil {
		return err
	}
	if err = decodeRecoveryRecords(journal, complete); err != nil {
		return err
	}
	if c.recoveryReadback != nil && c.recoveryReadback.pending != nil && c.recoveryReadback.pending.path == suffixPending {
		held, err := c.read(suffixPending, 4096, 0600, 0, 0)
		if err != nil || !sameInputFile(held.info, c.recoveryReadback.pending.info) || !bytes.Equal(held.body, recoveryPendingBytes(complete.body)) {
			return errors.Join(ErrBinding, err)
		}
		c.replacementPaths = append(c.replacementPaths, suffixPending)
	} else {
		if _, err = os.Lstat(suffixPending); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
		c.absent = append(c.absent, suffixPending)
	}
	c.replacementPaths = append(c.replacementPaths, suffix, suffixResult)
	c.recoveryNativeBytes = bytes.Clone(cut.Boundary.NativeContext)
	return nil
}
func decodeRecoveryRecords(journal, result *initialFile) error {
	if journal == nil || result == nil || len(journal.body) == 0 || journal.body[len(journal.body)-1] != '\n' {
		return ErrBinding
	}
	lines := bytes.Split(journal.body[:len(journal.body)-1], []byte{'\n'})
	if len(lines) != 9 {
		return ErrBinding
	}
	rows := make([]initialRecord, len(lines))
	for i, line := range lines {
		if json.Unmarshal(line, &rows[i]) != nil {
			return ErrBinding
		}
		raw, _ := json.Marshal(rows[i])
		if !bytes.Equal(raw, line) || rows[i].Sequence != i {
			return ErrBinding
		}
	}
	js, _ := infoSys(journal.info)
	rs, _ := infoSys(result.info)
	if js == nil || rs == nil {
		return ErrBinding
	}
	_, _, pending := recoveryPaths(journal.path[:len(journal.path)-len(".recovery-journal")])
	sha := digest(result.body)
	guard := digest(recoveryPendingBytes(result.body))
	wanted := []initialRecord{
		{Phase: "join-recovery-v1", Path: journal.path, Device: uint64(js.Dev), Inode: js.Ino, Mode: 0600},
		{Phase: "file-intent", Path: pending, SHA256: guard, Mode: 0600},
		{Phase: "file-born", Path: pending, Mode: 0600},
		{Phase: "file-ready", Path: pending, SHA256: guard, Mode: 0600},
		{Phase: "recovery-admitted", Path: result.path, SHA256: sha, Mode: 0600},
		{Phase: "file-intent", Path: result.path, SHA256: sha, Mode: 0600},
		{Phase: "file-born", Path: result.path, Device: uint64(rs.Dev), Inode: rs.Ino, Mode: 0600},
		{Phase: "file-ready", Path: result.path, Device: uint64(rs.Dev), Inode: rs.Ino, SHA256: sha, Mode: 0600},
		{Phase: "recovery-verified", Path: result.path, Device: uint64(rs.Dev), Inode: rs.Ino, SHA256: sha, Mode: 0600},
	}
	if rows[2].Device == 0 || rows[2].Inode == 0 || rows[2].Device != rows[3].Device || rows[2].Inode != rows[3].Inode {
		return ErrBinding
	}
	wanted[2].Device, wanted[2].Inode = rows[2].Device, rows[2].Inode
	wanted[3].Device, wanted[3].Inode = rows[3].Device, rows[3].Inode
	for i := range wanted {
		wanted[i].Sequence = i
		if wanted[i] != rows[i] {
			return ErrBinding
		}
	}
	return nil
}
func (s *recoveryPublication) owns(path string) bool {
	if s == nil || s.births == nil {
		return false
	}
	if s.births.journal != nil && s.births.journal.path == path {
		return true
	}
	for _, f := range s.births.files {
		if f.path == path {
			return true
		}
	}
	return false
}
func (c *completedInitial) checkRecoveryNative() error {
	if c.recoveryNative == nil {
		return nil
	}
	return c.recoveryNative.check()
}

// The actual retained sibling parent is used for both publication markers.
func (c *completedInitial) recoveryParent(path string) os.FileInfo {
	if d := c.parents.byPath[filepath.Dir(path)]; d != nil {
		return d.info
	}
	return nil
}
