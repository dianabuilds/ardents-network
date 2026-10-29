package forwarding

import (
	"errors"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
)

func validInitialSelectionBirth(birth *initialRecord, path string) bool {
	return birth != nil && birth.Sequence == 111 && birth.Phase == "file-born" && birth.Path == path && birth.Device != 0 && birth.Inode != 0 && birth.SHA256 == "" && birth.Mode == 0600 && birth.UID == 0 && birth.GID == 0
}

func (s *recoveryInitialReloadAttempt) openSelectionContinuation() error {
	u := s.selection
	path := filepath.Join(s.candidate.selection.binding.Request.InstallationRoot, "binding.json")
	if u == nil || u.original == nil || u.mutation != nil || s.original.cut.Boundary.Records != 112 || s.proof.selectionBirth != &u.birth || s.proof.selectionFile != u.original || !validInitialSelectionBirth(&u.birth, path) {
		return ErrBinding
	}
	parent := s.files.files.dirs[filepath.Dir(path)]
	if parent == nil {
		return ErrBinding
	}
	if err := s.observe(); err != nil {
		return err
	}
	var err error
	u.mutation, err = fixedfile.ContinueBirth(s.ctx, path, parent.info, u.original.info, s.candidate.selection.raw, 0644, 0)
	if err != nil {
		return err
	}
	if !sameInputFile(u.mutation.Identity(), u.original.info) {
		return ErrBinding
	}
	return s.observe()
}

func (s *recoveryInitialReloadAttempt) continueSelection() error {
	u := s.selection
	if initialSelectionCreationCut(s.original.cut.Boundary.Records) {
		if err := s.createSelection(); err != nil {
			return err
		}
	}
	if u == nil || u.original == nil || u.mutation == nil || u.ready || s.proof.selectionFile != u.original || s.publication.births == nil {
		return ErrBinding
	}
	if err := s.observe(); err != nil {
		return err
	}
	if s.original.cut.Boundary.Records == 112 {
		if s.proof.selectionBirth != &u.birth {
			return ErrBinding
		}
		if err := s.publication.births.record("continued-file-retained", u.original.path, u.original.info, digest(nil), 0600, 0, 0); err != nil {
			return err
		}
	} else if !initialSelectionCreationCut(s.original.cut.Boundary.Records) || s.proof.selectionCreation != &u.birth {
		return ErrBinding
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := u.commit(s.candidate.selection.raw, 0644); err != nil {
		return err
	}
	if err := errors.Join(observeInitialFile(u.original), s.observe()); err != nil {
		return err
	}
	if err := s.publication.births.record("continued-file-written", u.original.path, u.original.info, digest(u.original.body), 0644, 0, 0); err != nil {
		return err
	}
	return s.observe()
}

func (s *recoveryInitialReloadAttempt) createSelection() error {
	u := s.selection
	if u == nil || u.original != nil || u.mutation != nil || !initialSelectionCreationCut(s.original.cut.Boundary.Records) || s.publication.births == nil || s.proof.selectionBirth != nil || s.proof.selectionCreation != nil || s.proof.selectionFile != nil {
		return ErrBinding
	}
	path := filepath.Join(s.candidate.selection.binding.Request.InstallationRoot, "binding.json")
	parent := s.files.files.dirs[filepath.Dir(path)]
	if parent == nil {
		return ErrBinding
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.publication.births.record("continued-file-intent", path, nil, digest(s.candidate.selection.raw), 0644, 0, 0); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	var err error
	u.mutation, err = fixedfile.Create(s.ctx, path, parent.info, s.candidate.selection.raw, 0644, 0)
	if err != nil {
		return err
	}
	native, ok := infoSys(u.mutation.Identity())
	if !ok {
		return ErrBinding
	}
	u.birth = initialRecord{Sequence: 111, Phase: "file-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
	s.proof.selectionCreation = &u.birth
	if err := s.publication.births.record("continued-file-born", path, u.mutation.Identity(), digest(nil), 0600, 0, 0); err != nil {
		return err
	}
	if err := readInitialEmptyBirth(s.files.files, path, u.birth); err != nil {
		return err
	}
	u.original = s.files.files.files[path]
	s.proof.selectionFile = u.original
	return s.observe()
}
