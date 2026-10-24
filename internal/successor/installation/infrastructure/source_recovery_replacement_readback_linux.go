package infrastructure

import (
	"errors"
	"os"
	"syscall"
)

// A recovered replacement requires the original immutable prefix, genuine cut
// proof and a separate complete suffix. Ordinary readers own new SH leases;
// only the actual current recovery writer may read back its own blocked result.
func (s *sourceInitial) readReplacementCompletion(current *sourceSelection, journal string) ([]byte, error) {
	b := current.binding
	var cut *sourceRecoveryPrefix
	creating := s.transition != nil && s.transition.recovery != nil && s.transition.recovery.prefix.journal.path == journal
	if creating {
		w := s.transition.recovery
		if !w.finished || s.transition.closed || w.prefix.cut == nil {
			return nil, ErrBinding
		}
		cut = w.prefix
		if err := s.transition.check(); err != nil {
			return nil, err
		}
	} else {
		var err error
		cut, err = openSourceRecoveryPrefixCustody(s.ctx, journal, "replacement", false)
		if err != nil {
			return nil, err
		}
		s.cuts = append(s.cuts, cut)
		if err = syscall.Flock(int(cut.journal.file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
			return nil, err
		}
		if err = cut.readOriginalCut(b.Generation, digest(current.bindingBytes()), b.ProgramSHA256); err != nil {
			return nil, err
		}
	}
	original := current.files[journal]
	// This also applies to the actual current-writer exception. A genuine cut
	// plus a complete suffix cannot widen the permitted continuation boundary.
	if !sourceReplacementRecoveryPrefixSupported(cut.records) {
		return nil, ErrBinding
	}
	if original == nil || !os.SameFile(original.info, cut.journal.info) {
		return nil, ErrBinding
	}
	control, result, _ := sourceCutPaths(journal)
	for _, row := range []struct {
		path     string
		retained *sourceFile
	}{{control, cut.cutControl}, {result, cut.cutResult}} {
		if row.retained == nil {
			return nil, ErrBinding
		}
		if _, err := current.read(row.path, 64<<10, 0600, 0, 0); err != nil {
			return nil, err
		}
		if !os.SameFile(current.files[row.path].info, row.retained.info) {
			return nil, ErrBinding
		}
	}
	path := journal + ".recovery-completion"
	completion, err := current.read(path, 4<<20, 0600, 0, 0)
	if err != nil {
		return nil, err
	}
	if creating {
		if !os.SameFile(current.files[path].info, s.transition.journal.info) {
			return nil, ErrBinding
		}
	} else if err = syscall.Flock(int(current.files[path].file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		return nil, err
	}
	if _, err = decodeSourceCutResult(current.files[control].raw, current.files[result].raw, original.raw, current.files[control].info, current.files[result].info, original.info,
		journal, "replacement", b.Generation, digest(current.bindingBytes()), b.ProgramSHA256); err != nil {
		return nil, err
	}
	transcript, err := decodeSourceRecoveryCompletion(completion, original.raw, original.info, journal, "replacement", b.Generation, digest(current.bindingBytes()))
	if err != nil {
		return nil, err
	}
	if err = errors.Join(current.observe(), cut.check()); err != nil {
		return nil, err
	}
	for _, known := range []string{control, result, path} {
		s.replacementJournals[known] = true
	}
	return transcript, nil
}
