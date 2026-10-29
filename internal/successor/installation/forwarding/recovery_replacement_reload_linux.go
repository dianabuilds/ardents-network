package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// This closed byte inventory grants no stopped state or first Reload. Those
// require the genuine original cut, retained predecessor and fresh observers.
func replacementPreReloadInventory(prefix []byte, cut transitionCutResult, selected, previous generationSelection, hashes map[string]string, before []byte) (map[string]initialRecord, error) {
	rows, err := decodeTransitionPrefix(prefix, "replacement")
	b := selected.binding
	if err != nil || len(rows) != 57 && len(rows) != 58 || cut.Boundary.Records != uint32(len(rows)) || cut.Boundary.PrefixSHA256 != digest(prefix) || cut.Boundary.Operation != "replacement" || cut.Boundary.Journal != replacementJournal(b.Request.InstallationRoot, b.Generation) || cut.Boundary.Generation != b.Generation || cut.Boundary.BindingSHA256 != digest(selected.raw) || !bytes.Equal(cut.Boundary.Binding, selected.raw) || cut.Origin.ProgramSHA256 != b.ProgramSHA256 || !bytes.Equal(cut.Boundary.OriginalContext, before) || !bytes.Equal(cut.Boundary.NativeContext, before) {
		return nil, errors.Join(ErrRepairRequired, err)
	}
	native, err := decodeNativeContext(before)
	if err != nil || native.Manager != cut.Boundary.Manager {
		return nil, errors.Join(ErrBinding, err)
	}
	return decodeReplacementPreReloadRecords(prefix, selected, previous, hashes, before)
}

// The after triple belongs to this separate completion, never to the immutable
// original prefix. Its actual root writer still owes all I/O and close proofs.
func replacementReloadCompletion(original *recoveryOriginal, candidate *recoveryCandidate, previous generationSelection, hashes map[string]string, before, after *initialFile) ([]byte, error) {
	if original == nil || original.prefix == nil || candidate == nil || before == nil || after == nil {
		return nil, ErrInput
	}
	if _, err := replacementPreReloadInventory(original.prefix.journal.body, original.cut, candidate.selection, previous, hashes, before.body); err != nil {
		return nil, err
	}
	_, afterPath := replacementNativePaths(original.prefix.journal.path)
	native, ok := infoSys(after.info)
	if !ok || !nodeAccess(after.info, 0600, 0, 0, false) || after.path != afterPath || after.info.Size() != int64(len(after.body)) {
		return nil, ErrBinding
	}
	if err := compareReloadContexts(before.body, after.body); err != nil {
		return nil, err
	}
	b := candidate.selection.binding
	rows := []initialRecord{}
	if original.cut.Boundary.Records == 57 {
		rows = append(rows, initialRecord{Phase: "reload-intent", Path: "/etc/systemd/system/ardents-forwarding.service", SHA256: b.UnitSHA256, Mode: 0644})
	}
	sha := digest(after.body)
	rows = append(rows, initialRecord{Phase: "file-intent", Path: afterPath, SHA256: sha, Mode: 0600}, initialRecord{Phase: "file-born", Path: afterPath, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}, initialRecord{Phase: "file-ready", Path: afterPath, Device: uint64(native.Dev), Inode: native.Ino, SHA256: sha, Mode: 0600}, initialRecord{Phase: "replacement-verified", Path: filepath.Join(b.Request.InstallationRoot, "binding.json"), SHA256: digest(candidate.selection.raw), Mode: 0644})
	full := bytes.Clone(original.prefix.journal.body)
	delta := []json.RawMessage{}
	for n, row := range rows {
		row.Sequence = int(original.cut.Boundary.Records) + n
		raw, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		delta = append(delta, raw)
		full = append(full, raw...)
		full = append(full, '\n')
	}
	if _, err := decodeReplacementContextRecords(full, candidate.selection, previous, hashes, before.body, after.body); err != nil {
		return nil, err
	}
	cut := original.cut.Boundary
	completion := recoveryCompletion{Schema: "ardents-forwarding-recovery-completion-v2", Operation: "replacement", Journal: cut.Journal, Device: cut.Device, Inode: cut.Inode, PrefixSHA256: digest(original.prefix.journal.body), PrefixRecords: cut.Records, Generation: cut.Generation, BindingSHA256: cut.BindingSHA256, OriginalCutSHA256: digest(original.result.body), Outcome: "selected-stopped", Records: delta, TranscriptSHA256: digest(full), NativeContextSHA256: sha}
	raw, err := json.Marshal(completion)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if _, _, err := decodeRecoveryCompletion(raw, original.prefix.journal.body, original.result.body, original.cut); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *recoveryReplacementAttempt) runBeforeReload(bindingPath string, previous generationSelection, hashes map[string]string) error {
	c := s.history.completed.original
	if s.beforeReload == nil || s.beforeReload.original != s.original || s.beforeReload.owner != c.files {
		return ErrBinding
	}
	if _, err := replacementPreReloadInventory(s.original.prefix.journal.body, s.original.cut, s.candidate.selection, previous, hashes, s.beforeReload.beforeReload.body); err != nil {
		return err
	}
	var err error
	s.semantic, err = inspectReplacement(s.ctx, c)
	if err != nil {
		return err
	}
	s.native, err = openRecoveryContext(s.ctx, s.original.cut.Boundary.NativeContext)
	if err != nil {
		return err
	}
	s.manager, err = systemd.OpenForwardingRecoveryReplacement(s.ctx, s.native.expected.Manager)
	if err != nil {
		return err
	}
	b := c.binding
	s.publication.originalPending = c.files.files[replacementPending(b.Request.InstallationRoot)]
	s.publication.originalParent = c.files.dirs[b.Request.InstallationRoot].info
	if s.publication.originalPending == nil {
		return ErrBinding
	}
	s.publication.replacementReload = s
	c.files.recoveryPublishing = s.publication
	if err := s.observe(); err != nil {
		return err
	}
	cut := s.original.cut.Boundary
	s.publication.completion = recoveryCompletion{Schema: "ardents-forwarding-recovery-completion-v2", Operation: "replacement", Journal: cut.Journal, Generation: b.Generation, BindingSHA256: cut.BindingSHA256, OriginalCutSHA256: digest(s.original.result.body)}
	sealed := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	unit := c.files.files["/etc/systemd/system/ardents-forwarding.service"]
	if unit == nil || c.files.dirs[sealed] == nil {
		return ErrBinding
	}
	selected, err := s.publication.begin(s.ctx, s.original, c.files.dirs[sealed].info, unit.info, s.observe, nil)
	if err != nil {
		return err
	}
	beforePath, afterPath, err := recoveryReloadProofPaths(cut.Journal, "replacement")
	if err != nil {
		return err
	}
	if err := s.publication.births.write(beforePath, cut.NativeContext, 0600, 0, 0, s.observe); err != nil {
		return err
	}
	s.proof.before = s.publication.births.files[len(s.publication.births.files)-1]
	if err := errors.Join(s.observe(), s.publication.births.record("recovery-reload-intent", unit.path, nil, b.UnitSHA256, 0644, 0, 0)); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.native.close(); err != nil {
		return err
	}
	s.native = nil
	if err := s.manager.Reload(); err != nil {
		return err
	}
	return s.finishBeforeReload(bindingPath, selected, previous, hashes, unit, afterPath)
}

func (s *recoveryReplacementAttempt) finishBeforeReload(bindingPath string, selected, previous generationSelection, hashes map[string]string, unit *initialFile, afterPath string) (returned error) {
	before, err := decodeNativeContext(s.original.cut.Boundary.NativeContext)
	if err != nil {
		return err
	}
	namespace, err := openStartNamespace(s.ctx, before.UnitPaths)
	if namespace != nil {
		defer func() { returned = errors.Join(returned, namespace.close()) }()
	}
	if err != nil {
		return err
	}
	after, err := nativeContextAfterReload(s.original.cut.Boundary.NativeContext, namespace)
	if err != nil {
		return err
	}
	s.native, err = openRecoveryContext(s.ctx, after)
	if err != nil {
		return err
	}
	c := s.history.completed.original
	// The same operation's old generation observer admits only this actual
	// owned Reload's generator refresh, retaining late native Close failures.
	if err := c.files.observeOwnedReload(namespace); err != nil {
		return err
	}
	if err := namespace.close(); err != nil {
		return err
	}
	s.after = after
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.publication.births.write(afterPath, after, 0600, 0, 0, s.observe); err != nil {
		return err
	}
	s.proof.after = s.publication.births.files[len(s.publication.births.files)-1]
	raw, err := replacementReloadCompletion(s.original, s.candidate, previous, hashes, s.beforeReload.beforeReload, s.proof.after)
	if err != nil {
		return err
	}
	completion, _, err := decodeRecoveryCompletion(raw, s.original.prefix.journal.body, s.original.result.body, s.original.cut)
	if err != nil {
		return err
	}
	s.publication.raw, s.publication.completion = raw, completion
	sealed := filepath.Join(selected.binding.Request.InstallationRoot, "generations", selected.binding.Generation)
	if err := s.publication.finish(s.original, selected, c.files.dirs[sealed].info, unit.info, nil, s.observe); err != nil {
		return err
	}
	if err := errors.Join(s.observe(), s.publication.close(), s.manager.Close(), s.native.close(), s.history.close(), s.original.close()); err != nil {
		return err
	}
	if err := readbackRecoveryStopped(s.ctx, s.v, s.a, s.input, s.publication, bindingPath); err != nil {
		return err
	}
	return errors.Join(s.semantic.check(), s.input.Check(), s.a.Check(s.ctx, s.v))
}
