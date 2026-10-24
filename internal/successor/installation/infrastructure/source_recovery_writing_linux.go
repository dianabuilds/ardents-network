package infrastructure

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"syscall"
)

// sourceRecoveryWriting belongs to a fresh sourceBirths lifetime. Its projected
// transcript contains the original immutable prefix plus only the new records
// emitted by that writer. Framing completion confers no native/Release/Start
// authority; its coordinator still independently proves resources and joins.
type sourceRecoveryWriting struct {
	prefix     *sourceRecoveryPrefix
	borrowed   bool
	finished   bool
	metadata   sourceRecoveryCompletion
	records    []sourceRecord
	transcript []byte
}

// Acquisition returns its partial owner even on error, just like the ordinary
// birth writer. The caller must join Close. Existing completion evidence is
// never reopened for append, replaced, deleted or adopted by another attempt.
func openSourceRecoveryBirths(prefix *sourceRecoveryPrefix, generation, binding string, allowed map[string]bool) (*sourceBirths, error) {
	s := &sourceBirths{allowed: allowed, directories: map[string]os.FileInfo{}, directoryFiles: map[string]*os.File{}}
	if prefix == nil || prefix.ctx == nil || !prefix.exclusive || !sourceInitialSHA(generation) || !sourceInitialSHA(binding) {
		return s, ErrInput
	}
	s.ctx = prefix.ctx
	if err := prefix.begin(); err != nil {
		return s, err
	}
	w := &sourceRecoveryWriting{prefix: prefix, borrowed: true}
	s.recovery = w
	if err := prefix.check(); err != nil {
		return s, err
	}
	if prefix.completion != nil || len(prefix.records) == 0 ||
		(prefix.operation == "initial" && prefix.records[0].Phase != sourceRecoverableInitialPhase) {
		return s, ErrBinding
	}
	count := sourceRecoveryCompleteCount(prefix.operation, prefix.records[0].Phase)
	if count == 0 || len(prefix.records) > count {
		return s, ErrBinding
	}
	s.firstPhase, s.sequence = prefix.records[0].Phase, len(prefix.records)
	w.transcript = bytes.Clone(prefix.journal.raw)
	native := prefix.journal.info.Sys().(*syscall.Stat_t)
	w.metadata = sourceRecoveryCompletion{Schema: "ardents-source-recovery-completion-v1", Operation: prefix.operation,
		Journal: prefix.journal.path, Device: uint64(native.Dev), Inode: native.Ino,
		PrefixSHA256: digest(prefix.journal.raw), PrefixRecords: len(prefix.records),
		Generation: generation, BindingSHA256: binding, Records: sourceRecoveryRecords{}}
	header, err := json.Marshal(w.metadata)
	if err != nil {
		return s, err
	}
	// Use the same closed codec's field order. Only the records array and final
	// transcript digest remain open while actual continuation effects happen.
	ending := []byte(`[],"transcript_sha256":""}`)
	if !bytes.HasSuffix(header, ending) {
		return s, ErrBinding
	}
	header = append(bytes.Clone(header[:len(header)-len(ending)]), '[')
	path := prefix.journal.path + ".recovery-completion"
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return s, err
	}
	s.journal = &sourceBirth{path: path, file: file}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return s, err
	}
	s.journal.info, err = file.Stat()
	if err != nil || !sourceAccess(s.journal.info, 0600, 0, 0, false) || s.journal.info.Size() != 0 {
		return s, errors.Join(ErrBinding, err)
	}
	var fs syscall.Statfs_t
	if err = syscall.Fstatfs(int(file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
		return s, errors.Join(ErrBinding, err)
	}
	if err = errors.Join(file.Sync(), syncSourceParent(path)); err != nil {
		return s, err
	}
	if err = prefix.check(); err != nil {
		return s, err
	}
	return s, s.appendJournal(header)
}

func (w *sourceRecoveryWriting) beforeRecord(s *sourceBirths) error {
	if w == nil || w.finished || s.sequence != w.metadata.PrefixRecords+len(w.records) ||
		s.sequence >= sourceRecoveryCompleteCount(w.metadata.Operation, s.firstPhase) {
		return ErrBinding
	}
	return w.prefix.check()
}

// finishSourceRecoveryRecords closes only the envelope's complete byte grammar.
// The caller must first perform its genuine fresh native/semantic/Release
// observations, and must keep the original pending guard through every Close.
// Ordinary consumers independently reopen both files and validate those facts.
func (s *sourceBirths) finishSourceRecoveryRecords(binding []byte, hashes map[string]string, admitted, previousUnit string) (err error) {
	if s == nil || s.recovery == nil || s.closed {
		return ErrInput
	}
	defer func() {
		if err != nil && s.terminal == nil {
			s.terminal = err
		}
	}()
	w := s.recovery
	if w.finished || digest(binding) != w.metadata.BindingSHA256 {
		return ErrBinding
	}
	configuration, _, err := decodeSourceSelectedBinding(binding)
	if err != nil || configuration.Generation != w.metadata.Generation {
		return errors.Join(ErrBinding, err)
	}
	if _, err = decodeSourceRecoveredRecords(w.transcript, binding, w.metadata.Operation, hashes, admitted, previousUnit); err != nil {
		return err
	}
	if err = s.check(); err != nil {
		return err
	}
	trailer := []byte(`],"transcript_sha256":"` + digest(w.transcript) + "\"}\n")
	if err = s.appendJournal(trailer); err != nil {
		return err
	}
	w.finished = true
	// Independently consume the complete on-disk envelope framing before the
	// coordinator's later physical joins/publication. A failed read retains it.
	transcript, err := decodeSourceRecoveryCompletion(s.journal.body, w.prefix.journal.raw, w.prefix.journal.info,
		w.metadata.Journal, w.metadata.Operation, w.metadata.Generation, w.metadata.BindingSHA256)
	if err != nil || !bytes.Equal(transcript, w.transcript) {
		return errors.Join(ErrBinding, err)
	}
	return s.check()
}
