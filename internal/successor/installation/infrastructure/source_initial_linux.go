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
	ctx       context.Context
	verifier  *release.Verifier
	selection *sourceSelection
	history   *sourceHistory
	terminal  error
	closed    bool
}

func openSourceInitial(ctx context.Context, v *release.Verifier, binding, history string) (result *sourceInitial, err error) {
	if ctx == nil || v == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	s := &sourceInitial{ctx: ctx, verifier: v}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.close())
			result = nil
		}
	}()
	if s.selection, err = openSourceSelection(ctx, binding); err != nil {
		return nil, err
	}
	b := s.selection.binding
	if s.history, err = openSourceHistory(ctx, v, history); err != nil {
		return nil, err
	}
	journal := b.Request.InstallationRoot + ".source-initial-journal"
	raw, err := s.selection.read(journal, 4<<20, 0600, 0, 0)
	if err != nil {
		return nil, err
	}
	admitted, err := s.history.commitment(v, s.selection.files[binding].raw)
	if err != nil {
		return nil, err
	}
	hashes := make(map[string]string, len(s.selection.files))
	for path, file := range s.selection.files {
		hashes[path] = digest(file.raw)
	}
	recorded, err := decodeSourceInitialRecords(raw, b, hashes, admitted)
	if err != nil {
		return nil, err
	}
	// Account database postimages are historical helper evidence, not an
	// immutable Source-owned host database. The initial creator still checks
	// its actual helper/account custody; a later start admits the current
	// bound account separately. Semantic root contents remain State-owned.
	for path, record := range recorded {
		info, directory := s.selection.identities[path]
		if !directory {
			file := s.selection.files[path]
			if file == nil {
				return nil, ErrBinding
			}
			info = file.info
		}
		if !sourceAccess(info, os.FileMode(record.Mode), record.UID, record.GID, directory) {
			return nil, ErrBinding
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || uint64(native.Dev) != record.Device || native.Ino != record.Inode {
			return nil, ErrBinding
		}
	}
	return s, s.check()
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
	return s.history.check(s.verifier)
}

func (s *sourceInitial) close() error {
	if s == nil {
		return nil
	}
	if !s.closed {
		s.closed = true
		s.terminal = errors.Join(s.terminal, s.selection.close(), s.history.close(), s.ctx.Err())
	}
	return s.terminal
}
