package forwarding

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
)

// Timestamp is the final generation artifact. This concrete continuation owns
// only its original empty birth or this attempt's new exclusive birth.
func validInitialTimestampBirth(birth *initialRecord, path string) bool {
	return birth != nil && birth.Sequence == 106 && birth.Phase == "file-born" && birth.Path == path && birth.Device != 0 && birth.Inode != 0 && birth.SHA256 == "" && birth.Mode == 0600 && birth.UID == 0 && birth.GID == 0
}

func (s *recoveryInitialFiles) checkGeneration(candidate *recoveryCandidate) error {
	b := candidate.selection.binding
	directory := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	u := s.artifact
	if u == nil {
		return verifyInitialInventory(directory, candidate.files)
	}
	path := filepath.Join(directory, "timestamp.json")
	if len(candidate.files) != 12 || u.original != s.files.files[path] {
		return ErrBinding
	}
	if u.original == nil {
		if u.mutation != nil || u.ready {
			return ErrBinding
		}
		if err := s.files.absent(path); err != nil {
			return err
		}
	} else {
		if !validInitialTimestampBirth(&u.birth, path) || u.mutation != nil && !os.SameFile(u.mutation.Identity(), u.original.info) {
			return ErrBinding
		}
		mode, wanted := os.FileMode(0600), []byte(nil)
		if u.ready {
			if u.mutation == nil {
				return ErrBinding
			}
			mode, wanted = 0444, candidate.files["timestamp.json"]
		}
		if !nodeAccess(u.original.info, mode, 0, 0, false) || !bytes.Equal(u.original.body, wanted) {
			return ErrBinding
		}
		if err := observeInitialFile(u.original); err != nil {
			return err
		}
	}
	names := map[string]bool{}
	for name, wanted := range candidate.files {
		if name == "timestamp.json" {
			if u.original != nil {
				names[name] = true
			}
			continue
		}
		file := s.files.files[filepath.Join(directory, name)]
		mode := os.FileMode(0444)
		if name == programName {
			mode = 0555
		}
		if file == nil || !nodeAccess(file.info, mode, 0, 0, false) || !bytes.Equal(file.body, wanted) {
			return ErrBinding
		}
		if err := observeInitialFile(file); err != nil {
			return err
		}
		names[name] = true
	}
	dir := s.files.dirs[directory]
	if dir == nil {
		return ErrBinding
	}
	if _, err := dir.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	entries, err := dir.file.ReadDir(len(names) + 1)
	if err != nil && err != io.EOF || len(entries) != len(names) {
		return errors.Join(ErrBinding, err)
	}
	for _, entry := range entries {
		if !names[entry.Name()] || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return ErrBinding
		}
	}
	return s.files.links()
}

func (s *recoveryInitialReloadAttempt) openTimestampContinuation() error {
	u := s.artifact
	path := filepath.Join(s.candidate.selection.binding.Request.InstallationRoot, "generations", s.candidate.selection.binding.Generation, "timestamp.json")
	if u == nil || u.original == nil || u.mutation != nil || s.original.cut.Boundary.Records != 107 || s.proof.artifactBirth != &u.birth || s.proof.artifactFile != u.original || !validInitialTimestampBirth(&u.birth, path) {
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
	u.mutation, err = fixedfile.ContinueBirth(s.ctx, path, parent.info, u.original.info, s.candidate.files["timestamp.json"], 0444, 0)
	if err != nil {
		return err
	}
	if !sameInputFile(u.mutation.Identity(), u.original.info) {
		return ErrBinding
	}
	return s.observe()
}

func (s *recoveryInitialReloadAttempt) createTimestamp() error {
	u := s.artifact
	if u == nil || u.original != nil || u.mutation != nil || s.original.cut.Boundary.Records < 105 || s.original.cut.Boundary.Records > 106 || s.publication.births == nil || s.proof.artifactBirth != nil || s.proof.artifactCreation != nil || s.proof.artifactFile != nil {
		return ErrBinding
	}
	path := filepath.Join(s.candidate.selection.binding.Request.InstallationRoot, "generations", s.candidate.selection.binding.Generation, "timestamp.json")
	parent := s.files.files.dirs[filepath.Dir(path)]
	if parent == nil {
		return ErrBinding
	}
	wanted := s.candidate.files["timestamp.json"]
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.publication.births.record("continued-artifact-intent", path, nil, digest(wanted), 0444, 0, 0); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	var err error
	u.mutation, err = fixedfile.Create(s.ctx, path, parent.info, wanted, 0444, 0)
	if err != nil {
		return err
	}
	native, ok := infoSys(u.mutation.Identity())
	if !ok {
		return ErrBinding
	}
	u.birth = initialRecord{Sequence: 106, Phase: "file-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
	s.proof.artifactCreation = &u.birth
	if err := s.publication.births.record("continued-artifact-born", path, u.mutation.Identity(), digest(nil), 0600, 0, 0); err != nil {
		return err
	}
	if err := readInitialEmptyBirth(s.files.files, path, u.birth); err != nil {
		return err
	}
	u.original = s.files.files.files[path]
	s.proof.artifactFile = u.original
	return s.observe()
}

func (s *recoveryInitialReloadAttempt) continueTimestamp() error {
	u := s.artifact
	if s.original.cut.Boundary.Records <= 106 {
		if err := s.createTimestamp(); err != nil {
			return err
		}
	}
	if u == nil || u.original == nil || u.mutation == nil || u.ready || s.proof.artifactFile != u.original || s.publication.births == nil {
		return ErrBinding
	}
	if err := s.observe(); err != nil {
		return err
	}
	if s.original.cut.Boundary.Records == 107 {
		if s.proof.artifactBirth != &u.birth {
			return ErrBinding
		}
		if err := s.publication.births.record("continued-artifact-retained", u.original.path, u.original.info, digest(nil), 0600, 0, 0); err != nil {
			return err
		}
	} else if s.proof.artifactCreation != &u.birth {
		return ErrBinding
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := u.commit(s.candidate.files["timestamp.json"], 0444); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.publication.births.record("continued-artifact-written", u.original.path, u.original.info, digest(u.original.body), 0444, 0, 0); err != nil {
		return err
	}
	return s.observe()
}
