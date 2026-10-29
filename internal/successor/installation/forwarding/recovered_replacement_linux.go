package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// This historical proof owns no writer or semantic permission. Its files are
// independently retained by completedFiles. Only an active recovery can attach
// its own freshly acquired original EX lease while its suffix is unpublished.
type recoveredReplacement struct {
	generation          string
	journal, completion *initialFile
	logical, nativeRaw  []byte
	paths               []string
	original            *recoveryOriginal
	beforeReload        *initialFile
	owner               *completedFiles
}

func (r *recoveredReplacement) check() error {
	if r == nil || r.journal == nil || len(r.logical) == 0 || len(r.nativeRaw) == 0 {
		return ErrBinding
	}
	if r.original != nil {
		if r.completion != nil || r.original.prefix == nil || !sameInputFile(r.journal.info, r.original.prefix.journal.info) || !bytes.Equal(r.journal.body, r.original.prefix.journal.body) {
			return ErrBinding
		}
		if r.beforeReload != nil {
			if err := r.checkBeforeReloadAfter(); err != nil {
				return err
			}
		}
		return r.original.check()
	}
	if r.completion == nil {
		return ErrBinding
	}
	return nil
}

func (r *recoveredReplacement) checkBeforeReloadAfter() error {
	if r.owner == nil || r.original == nil || r.beforeReload == nil || !bytes.Equal(r.beforeReload.body, r.original.cut.Boundary.NativeContext) {
		return ErrBinding
	}
	_, after := replacementNativePaths(r.journal.path)
	p := r.owner.recoveryPublishing
	if p != nil && p.replacementReload != nil && p.replacementReload.beforeReload == r && p.replacementReload.original == r.original && p.births != nil && p.completion.Journal == r.journal.path && p.births.allowed[after] {
		for _, file := range p.births.files {
			if file.path == after {
				return p.births.check()
			}
		}
	}
	if _, err := os.Lstat(after); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

func (r *recoveredReplacement) clear() {
	if r != nil {
		clear(r.logical)
		clear(r.nativeRaw)
	}
}

func stoppedReplacementTranscript(prefix []byte, cut transitionCutResult, selected, previous generationSelection, hashes map[string]string, before, after []byte) ([]byte, error) {
	rows, err := decodeTransitionPrefix(prefix, "replacement")
	if err != nil || len(rows) != 61 && len(rows) != 62 || rows[0].Phase != "forwarding-replacement-v2" || cut.Boundary.Operation != "replacement" || !bytes.Equal(cut.Boundary.Binding, selected.raw) || cut.Origin.ProgramSHA256 != selected.binding.ProgramSHA256 || !bytes.Equal(cut.Boundary.OriginalContext, before) || !bytes.Equal(cut.Boundary.NativeContext, after) || rows[2].SHA256 != digest(before) {
		return nil, errors.Join(ErrRepairRequired, err)
	}
	for _, raw := range [][]byte{before, after} {
		context, err := decodeNativeContext(raw)
		if err != nil || context.Manager != cut.Boundary.Manager {
			return nil, errors.Join(ErrBinding, err)
		}
	}
	logical := bytes.Clone(prefix)
	if len(rows) == 61 {
		row := initialRecord{Sequence: 61, Phase: "replacement-verified", Path: filepath.Join(selected.binding.Request.InstallationRoot, "binding.json"), SHA256: digest(selected.raw), Mode: 0644}
		raw, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		logical = append(logical, raw...)
		logical = append(logical, '\n')
	}
	if _, err := decodeReplacementContextRecords(logical, selected, previous, hashes, before, after); err != nil {
		return nil, err
	}
	return logical, nil
}

func (f *completedFiles) recoveryFile(path string, maximum int64, exclusive bool) (*initialFile, error) {
	if original := f.files[path]; original != nil {
		if exclusive || !nodeAccess(original.info, 0600, 0, 0, false) || original.info.Size() > maximum {
			return nil, ErrBinding
		}
		return original, observeInitialFile(original)
	}
	file, err := f.retain(path, maximum, 0600, 0, 0)
	if err != nil {
		return nil, err
	}
	if exclusive {
		if err := syscall.Flock(int(file.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			return nil, err
		}
	}
	_, err = f.fill(file, maximum)
	return file, err
}

func (c *completedInitial) readRecoveredReplacement(file *initialFile, selected, previous generationSelection, hashes map[string]string, before, after []byte) ([]byte, error) {
	f := c.files
	control, result, pending := recoveryPaths(file.path)
	cutControl, cutResult, cutPending := transitionCutPaths(file.path)
	any := false
	for _, path := range []string{control, result, pending, cutControl, cutResult, cutPending, file.path + ".recovery-native-context"} {
		if _, err := os.Lstat(path); err == nil {
			any = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if !any {
		return file.body, nil
	}
	r := &recoveredReplacement{generation: selected.binding.Generation, journal: file}
	f.recoveredReplacements = append(f.recoveredReplacements, r)
	cc, err := f.recoveryFile(cutControl, 64<<10, true)
	if err != nil {
		return nil, err
	}
	cr, err := f.recoveryFile(cutResult, 256<<10, false)
	if err != nil {
		return nil, err
	}
	cut, err := decodeTransitionCut(cc.body, cr.body, file.body, cc.info, cr.info, file.info)
	if err != nil {
		return nil, err
	}
	preReload := cut.Boundary.Records == 57 || cut.Boundary.Records == 58
	var logical []byte
	if preReload {
		if _, err := replacementPreReloadInventory(file.body, cut, selected, previous, hashes, before); err != nil {
			return nil, err
		}
	} else {
		logical, err = stoppedReplacementTranscript(file.body, cut, selected, previous, hashes, before, after)
		if err != nil {
			return nil, err
		}
	}
	for _, path := range []string{cutPending, file.path + ".recovery-native-context"} {
		if err := f.absent(path); err != nil {
			return nil, err
		}
		f.absentPaths = append(f.absentPaths, path)
	}
	r.logical, r.nativeRaw = logical, bytes.Clone(cut.Boundary.NativeContext)
	r.paths = []string{cutControl, cutResult}
	if f.interrupted != nil && f.interrupted.cut.Boundary.Journal == file.path {
		if err := f.interrupted.check(); err != nil {
			return nil, err
		}
		if !sameInputFile(file.info, f.interrupted.prefix.journal.info) || !bytes.Equal(file.body, f.interrupted.prefix.journal.body) || !bytes.Equal(cr.body, f.interrupted.result.body) {
			return nil, ErrBinding
		}
		if err := recoveryAbsent(file.path); err != nil {
			return nil, err
		}
		r.original = f.interrupted
		if preReload {
			beforePath, _ := replacementNativePaths(file.path)
			r.beforeReload = f.files[beforePath]
			r.owner = f
			r.logical = bytes.Clone(file.body)
			if r.beforeReload == nil || after != nil {
				return nil, ErrBinding
			}
		}
		return r.logical, r.check()
	}
	rc, err := f.recoveryFile(control, 64<<10, true)
	if err != nil {
		return nil, err
	}
	r.completion, err = f.recoveryFile(result, 4<<20, false)
	if err != nil {
		return nil, err
	}
	envelope, reconstructed, err := decodeRecoveryCompletion(r.completion.body, file.body, cr.body, cut)
	if err != nil || !preReload && !bytes.Equal(reconstructed, logical) {
		return nil, errors.Join(ErrBinding, err)
	}
	var reload *recoveryReloadProof
	if preReload {
		if envelope.Schema != "ardents-forwarding-recovery-completion-v2" || digest(after) != envelope.NativeContextSHA256 {
			return nil, ErrBinding
		}
		beforePath, afterPath, err := recoveryReloadProofPaths(file.path, "replacement")
		if err != nil {
			return nil, err
		}
		reload = &recoveryReloadProof{}
		reload.before, err = f.recoveryFile(beforePath, 64<<10, false)
		if err != nil {
			return nil, err
		}
		reload.after = f.files[afterPath]
		if reload.after == nil || !bytes.Equal(reload.before.body, before) || !bytes.Equal(reload.after.body, after) {
			return nil, ErrBinding
		}
		if _, err := decodeReplacementContextRecords(reconstructed, selected, previous, hashes, before, after); err != nil {
			return nil, err
		}
		_, unused := recoveryReloadPaths(file.path)
		if err := f.absent(unused); err != nil {
			return nil, err
		}
		f.absentPaths = append(f.absentPaths, unused)
		r.logical = bytes.Clone(reconstructed)
		r.nativeRaw = bytes.Clone(after)
		r.paths = append(r.paths, beforePath)
	} else if envelope.Schema != "ardents-forwarding-recovery-completion-v1" {
		return nil, ErrBinding
	}
	sealed := filepath.Join(selected.binding.Request.InstallationRoot, "generations", selected.binding.Generation)
	unit := f.files["/etc/systemd/system/ardents-forwarding.service"]
	if unit == nil || f.dirs[sealed] == nil {
		return nil, ErrBinding
	}
	var pendingInfo os.FileInfo
	known := f.recoveryReadback
	if known != nil && known.completion.Journal == file.path {
		if known.births == nil || !known.births.closed || known.births.terminal != nil || known.pending == nil || known.originalPending == nil || !bytes.Equal(known.raw, r.completion.body) {
			return nil, ErrBinding
		}
		guard, err := f.recoveryFile(pending, 128, false)
		if err != nil || !sameInputFile(guard.info, known.pending.info) || !bytes.Equal(guard.body, recoveryPendingBytes(envelope.OriginalCutSHA256)) {
			return nil, errors.Join(ErrBinding, err)
		}
		originalPending, err := f.recoveryFile(replacementPending(selected.binding.Request.InstallationRoot), 128, false)
		if err != nil || !sameInputFile(originalPending.info, known.originalPending.info) || !bytes.Equal(originalPending.body, replacementPendingBytes(selected.raw)) {
			return nil, errors.Join(ErrBinding, err)
		}
		pendingInfo = guard.info
		r.paths = append(r.paths, pending)
	} else {
		if err := f.absent(pending); err != nil {
			return nil, err
		}
		f.absentPaths = append(f.absentPaths, pending)
	}
	if err := decodeRecoveryControlProof(rc.body, r.completion.body, envelope, selected, rc.info, r.completion.info, file.info, f.dirs[sealed].info, unit.info, pendingInfo, nil, reload); err != nil {
		return nil, err
	}
	r.paths = append(r.paths, control, result)
	return r.logical, r.check()
}

func (c *completedInitial) replacementPreReload(journal string) *recoveredReplacement {
	if c == nil || c.files == nil || c.files.interrupted == nil {
		return nil
	}
	for _, proof := range c.files.recoveredReplacements {
		if proof.journal != nil && proof.journal.path == journal && proof.original == c.files.interrupted && proof.beforeReload != nil {
			return proof
		}
	}
	return nil
}
