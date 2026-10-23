package infrastructure

import (
	"context"
	"errors"
	"os"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// sourceInitial independently retains a complete stopped birth transcript and
// its Source-owned resources. It holds no writer lease, creates no resource,
// initializes no semantic history, and grants no account, Release or Start right.
// The caller serializes its checks and joins them before physical close.
type sourceInitial struct {
	ctx                 context.Context
	verifier            *release.Verifier
	selection           *sourceSelection
	history             *sourceHistory
	terminal            error
	closed              bool
	lineage             []*sourceSelection
	replacementJournals map[string]bool
	firstAttempt        sourceAttemptNumber
	priorRetirement     string
	previousStart       *sourceStartHistory
	previousSelection   *sourceSelection
	transition          *sourceBirths
	birthJournal        *sourceFile
}

func openSourceInitial(ctx context.Context, v *release.Verifier, binding, history string) (result *sourceInitial, err error) {
	return openSourceInitialTransition(ctx, v, binding, history, nil)
}

// The original replacement writer may synchronously inspect its own complete
// blocked transcript before releasing physical custody. Ordinary consumers
// supply no writer and must independently acquire every completed journal.
func openSourceInitialTransition(ctx context.Context, v *release.Verifier, binding, history string, transition *sourceBirths) (result *sourceInitial, err error) {
	if ctx == nil || v == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	s := &sourceInitial{ctx: ctx, verifier: v, transition: transition}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.close())
			result = nil
		}
	}()
	if s.selection, err = openSourceSelection(ctx, binding); err != nil {
		return nil, err
	}
	if s.history, err = openSourceHistory(ctx, v, history); err != nil {
		return nil, err
	}
	if s.selection.replacement != nil {
		if err = s.readReplacementLineage(); err != nil {
			return nil, err
		}
	} else if err = s.readInitialBirth(s.selection); err != nil {
		return nil, err
	}
	return s, s.check()
}

// Historical selected binding/unit bytes come from the immutable generation;
// their original inode/access must still equal the active fixed files. Only a
// complete independently validated replacement transcript can account for
// changed active bytes. All other original births remain native observations.
func (s *sourceInitial) readInitialBirth(origin *sourceSelection) error {
	b := origin.binding
	journal := b.Request.InstallationRoot + ".source-initial-journal"
	raw, err := origin.read(journal, 4<<20, 0600, 0, 0)
	if err != nil {
		return err
	}
	s.birthJournal = origin.files[journal]
	admitted, err := s.history.commitment(s.verifier, origin.bindingBytes())
	if err != nil {
		return err
	}
	hashes := make(map[string]string, len(origin.files)+2)
	for path, file := range origin.files {
		hashes[path] = digest(file.raw)
	}
	selected := b.Request.InstallationRoot + "/binding.json"
	unit := "/etc/systemd/system/ardents-source.service"
	hashes[selected], hashes[unit] = digest(origin.bindingBytes()), b.UnitSHA256
	recorded, err := decodeSourceInitialRecords(raw, b, hashes, admitted)
	if err != nil {
		return err
	}
	// Account database postimages are historical helper evidence, not an
	// immutable Source-owned host database. The initial creator still checks
	// its actual helper/account custody; a later start admits the current
	// bound account separately. Semantic root contents remain State-owned.
	for path, record := range recorded {
		info, directory := origin.identities[path]
		if !directory {
			file := origin.files[path]
			if path == selected || path == unit {
				file = s.selection.files[path]
			}
			if file == nil {
				return ErrBinding
			}
			info = file.info
		}
		if !sourceAccess(info, os.FileMode(record.Mode), record.UID, record.GID, directory) {
			return ErrBinding
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || uint64(native.Dev) != record.Device || native.Ino != record.Inode {
			return ErrBinding
		}
	}
	return nil
}

func (s *sourceInitial) check() (err error) {
	if s == nil || s.ctx == nil || s.verifier == nil {
		return ErrInput
	}
	if s.terminal != nil {
		return s.terminal
	}
	if s.closed {
		return ErrInput
	}
	defer func() {
		err = errors.Join(err, s.ctx.Err())
		if err != nil {
			s.terminal = err
		}
	}()
	if err = s.history.check(s.verifier); err != nil {
		return err
	}
	if err = s.selection.observe(); err != nil {
		return err
	}
	for _, original := range s.lineage {
		if err = original.observe(); err != nil {
			return err
		}
	}
	if err = s.checkReplacementJournals(); err != nil {
		return err
	}
	return s.history.check(s.verifier)
}

func (s *sourceInitial) close() error {
	if s == nil {
		return nil
	}
	if !s.closed {
		s.closed = true
		for _, original := range s.lineage {
			s.terminal = errors.Join(s.terminal, original.close())
		}
		s.terminal = errors.Join(s.terminal, s.selection.close(), s.history.close(), s.ctx.Err())
	}
	return s.terminal
}
