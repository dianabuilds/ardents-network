package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// Historical child proof only. The snapshot owns every descriptor; an active
// recovery retains its own fresh original EX lease, never a Stop writer's lease.
type retirementRecovery struct {
	logical, nativeRaw []byte
	paths              []string
	completion, guard  *initialFile
	original           *recoveryOriginal
}

func (r *retirementRecovery) check(h *retirementSnapshot) error {
	if r == nil {
		return nil
	}
	if len(r.logical) == 0 || len(r.nativeRaw) == 0 || h == nil || h.journal == nil {
		return ErrBinding
	}
	_, _, cutPending := transitionCutPaths(h.journal.path)
	for _, path := range []string{cutPending, h.journal.path + ".recovery-native-context"} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	if r.original != nil {
		if r.completion != nil || !sameInputFile(h.journal.info, r.original.prefix.journal.info) || !bytes.Equal(h.journal.body, r.original.prefix.journal.body) {
			return ErrBinding
		}
		return r.original.check()
	}
	if r.completion == nil {
		return ErrBinding
	}
	if r.guard == nil {
		_, _, pending := recoveryPaths(h.journal.path)
		if _, err := os.Lstat(pending); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	} else if err := observeInitialFile(r.guard); err != nil {
		return err
	}
	return nil
}

// This classifier consumes only exact stopped original result-ready/final
// bytes. The caller still owes actual original-cut, native/resource/semantic
// observations and fresh Release before any separate completion publication.
func stoppedRetirementTranscript(prefix []byte, cut transitionCutResult, selected generationSelection, number attemptNumber, startSHA string, archive startArchive, id [16]byte, resultRaw []byte) ([]byte, error) {
	rows, err := decodeTransitionPrefix(prefix, "retirement")
	if err != nil || len(rows) != 17 && len(rows) != 18 || cut.Boundary.Operation != "retirement" || !bytes.Equal(cut.Boundary.Binding, selected.raw) || cut.Origin.ProgramSHA256 != selected.binding.ProgramSHA256 || number >= attemptLimit || cut.Boundary.Journal != number.retirementJournal(selected.binding.Request.InstallationRoot) {
		return nil, errors.Join(ErrRepairRequired, err)
	}
	if !bytes.Equal(cut.Boundary.OriginalContext, cut.Boundary.NativeContext) {
		return nil, ErrBinding
	}
	native, err := decodeNativeContext(cut.Boundary.NativeContext)
	if err != nil || native.Manager != cut.Boundary.Manager || native.Manager != archive.Manager {
		return nil, errors.Join(ErrBinding, err)
	}
	result, err := decodeRetirementResult(resultRaw, number, selected.binding, digest(selected.raw), startSHA, archive, id)
	if err != nil {
		return nil, err
	}
	logical := bytes.Clone(prefix)
	if len(rows) == 17 {
		ready := rows[16]
		row := retirementEntry{Schema: "ardents-forwarding-retirement-journal-v1", Sequence: 17, Phase: "retired", Path: filepath.Join(number.retirement(selected.binding.Request.InstallationRoot), "retirement-result.json"), Device: ready.Device, Inode: ready.Inode, SHA256: digest(resultRaw), Mode: 0600}
		raw, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		logical = append(logical, raw...)
		logical = append(logical, '\n')
	}
	challenge, err := retirementChallenge(result.Challenge)
	if err != nil {
		return nil, err
	}
	if _, err := decodeRetirementRecords(logical, cut.Boundary.Journal, number.retirement(selected.binding.Request.InstallationRoot), startSHA, digest(challenge[:]), result.CompletionSHA256, digest(resultRaw), selected.binding.GID); err != nil {
		return nil, err
	}
	return logical, nil
}

func (h *retirementSnapshot) readRecoveryFile(path string, maximum int64, exclusive bool) (*initialFile, error) {
	if h.files[path] != nil {
		return nil, ErrBinding
	}
	parent, err := h.parents.parents(path)
	if err != nil {
		return nil, err
	}
	file, err := openInputAt(parent, filepath.Base(path), false)
	if err != nil {
		return nil, err
	}
	owned := &initialFile{path: path, file: file}
	h.files[path] = owned
	h.order = append(h.order, owned)
	if exclusive {
		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			return nil, err
		}
	}
	return owned, h.readFile(owned, maximum)
}

func (h *retirementSnapshot) readRecovery(owner *completedInitial, original *recoveryOriginal, known *recoveryPublication) error {
	journal := h.journal.path
	control, result, pending := recoveryPaths(journal)
	cutControl, cutResult, cutPending := transitionCutPaths(journal)
	any := false
	for _, path := range []string{control, result, pending, cutControl, cutResult, cutPending, journal + ".recovery-native-context"} {
		if _, err := os.Lstat(path); err == nil {
			any = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if !any {
		if original != nil || known != nil {
			return ErrBinding
		}
		return nil
	}
	if owner == nil {
		return ErrRepairRequired
	}
	r := &retirementRecovery{}
	h.recovery = r
	cc, err := h.readRecoveryFile(cutControl, 64<<10, true)
	if err != nil {
		return err
	}
	cr, err := h.readRecoveryFile(cutResult, 256<<10, false)
	if err != nil {
		return err
	}
	cut, err := decodeTransitionCut(cc.body, cr.body, h.journal.body, cc.info, cr.info, h.journal.info)
	if err != nil {
		return err
	}
	selected, err := decodeSelection(cut.Boundary.Binding)
	if err != nil || selected.binding != h.binding || digest(selected.raw) != h.bindingSHA {
		return errors.Join(ErrBinding, err)
	}
	rawResult := h.files[filepath.Join(h.path, "retirement-result.json")].body
	r.logical, err = stoppedRetirementTranscript(h.journal.body, cut, selected, h.number, h.startSHA, h.archive, h.id, rawResult)
	if err != nil {
		return err
	}
	r.nativeRaw = bytes.Clone(cut.Boundary.NativeContext)
	r.paths = []string{cutControl, cutResult}
	if original != nil {
		if err := original.check(); err != nil {
			return err
		}
		if !sameInputFile(h.journal.info, original.prefix.journal.info) || !bytes.Equal(h.journal.body, original.prefix.journal.body) || !bytes.Equal(cr.body, original.result.body) || h.pending == nil {
			return ErrBinding
		}
		if err := recoveryAbsent(journal); err != nil {
			return err
		}
		r.original = original
		return r.check(h)
	}
	rc, err := h.readRecoveryFile(control, 64<<10, true)
	if err != nil {
		return err
	}
	r.completion, err = h.readRecoveryFile(result, 4<<20, false)
	if err != nil {
		return err
	}
	envelope, logical, err := decodeRecoveryCompletion(r.completion.body, h.journal.body, cr.body, cut)
	if err != nil || !bytes.Equal(logical, r.logical) {
		return errors.Join(ErrBinding, err)
	}
	sealed := filepath.Join(h.binding.Request.InstallationRoot, "generations", h.binding.Generation)
	unit := owner.files.files["/etc/systemd/system/ardents-forwarding.service"]
	if unit == nil || owner.files.dirs[sealed] == nil {
		return ErrBinding
	}
	var pendingInfo os.FileInfo
	if known != nil {
		if known.births == nil || !known.births.closed || known.births.terminal != nil || known.pending == nil || known.originalPending == nil || h.pending == nil || !bytes.Equal(known.raw, r.completion.body) || !sameInputFile(h.pending.info, known.originalPending.info) {
			return ErrBinding
		}
		r.guard, err = h.readRecoveryFile(pending, 128, false)
		if err != nil || !sameInputFile(r.guard.info, known.pending.info) || !bytes.Equal(r.guard.body, recoveryPendingBytes(envelope.OriginalCutSHA256)) {
			return errors.Join(ErrBinding, err)
		}
		pendingInfo = r.guard.info
		r.paths = append(r.paths, pending)
	}
	if err := decodeRecoveryControl(rc.body, r.completion.body, envelope, selected, rc.info, r.completion.info, h.journal.info, owner.files.dirs[sealed].info, unit.info, pendingInfo); err != nil {
		return err
	}
	r.paths = append(r.paths, control, result)
	return r.check(h)
}
