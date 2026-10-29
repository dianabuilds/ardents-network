package forwarding

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
)

type initialFileContinuation struct {
	original *initialFile
	birth    initialRecord
	mutation *fixedfile.Mutation
	ready    bool
}

func validInitialUnitCreation(birth *initialRecord) bool {
	return birth != nil && birth.Sequence == 115 && birth.Phase == "file-born" && birth.Path == "/etc/systemd/system/ardents-forwarding.service" && birth.Device != 0 && birth.Inode != 0 && birth.SHA256 == "" && birth.Mode == 0600 && birth.UID == 0 && birth.GID == 0
}

// Absence is checked under retained ancestors, then O_EXCL supplies the final
// physical exclusion. After birth only this operation's concrete mutation and
// independent reader may account for the new leaf.
func (s *recoveryInitialReloadAttempt) checkUnitContinuation() error {
	u := s.unit
	if u == nil || s.original.cut.Boundary.Records > 115 {
		return nil
	}
	path := "/etc/systemd/system/ardents-forwarding.service"
	if u.mutation == nil {
		if u.original != nil || s.proof.unitCreation != nil {
			return ErrBinding
		}
		return s.files.files.absent(path)
	}
	if u.original == nil || s.proof.unitCreation != &u.birth || !validInitialUnitCreation(&u.birth) || !os.SameFile(u.mutation.Identity(), u.original.info) || s.files.files.files[path] != u.original {
		return ErrBinding
	}
	return observeInitialFile(u.original)
}

func (s *recoveryInitialReloadAttempt) createUnit() error {
	u := s.unit
	if u == nil || u.original != nil || u.mutation != nil || !initialUnitCreationCut(s.original.cut.Boundary.Records) || s.publication.births == nil || s.proof.unitBirth != nil {
		return ErrBinding
	}
	path := "/etc/systemd/system/ardents-forwarding.service"
	parent := s.files.files.dirs[filepath.Dir(path)]
	if parent == nil {
		return ErrBinding
	}
	wanted := s.candidate.files["rendered-forwarding.service"]
	if err := errors.Join(s.observe(), s.publication.births.record("continued-file-intent", path, nil, digest(wanted), 0644, 0, 0)); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	var err error
	u.mutation, err = fixedfile.Create(s.ctx, path, parent.info, wanted, 0644, 0)
	if err != nil {
		return err
	}
	native, ok := infoSys(u.mutation.Identity())
	if !ok {
		return ErrBinding
	}
	u.birth = initialRecord{Sequence: 115, Phase: "file-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
	s.proof.unitCreation = &u.birth
	if err := s.publication.births.record("continued-file-born", path, u.mutation.Identity(), digest(nil), 0600, 0, 0); err != nil {
		return err
	}
	if err := readInitialEmptyBirth(s.files.files, path, u.birth); err != nil {
		return err
	}
	u.original = s.files.files.files[path]
	return s.observe()
}

// Physical empty-file custody only; the actual caller separately validates its
// exact unit/selection path and original phase. This grants no effect admission.
func readInitialEmptyBirth(files *completedFiles, path string, record initialRecord) error {
	if files == nil || files.files[path] != nil || record.Path != path || record.Phase != "file-born" || record.Mode != 0600 || record.UID != 0 || record.GID != 0 || record.SHA256 != "" {
		return ErrBinding
	}
	parent, err := files.parents(path)
	if err != nil {
		return err
	}
	file, err := openInputAt(parent, filepath.Base(path), false)
	if err != nil {
		return err
	}
	own := &initialFile{path: path, file: file}
	files.files[path] = own
	files.fileOrder = append(files.fileOrder, own)
	own.info, err = file.Stat()
	native, ok := infoSys(own.info)
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(file.Fd()), &fs)
	if err != nil || fsErr != nil || !ok || fs.Type != 0xef53 || fs.Flags&1 != 0 || !nodeAccess(own.info, 0600, 0, 0, false) || own.info.Size() != 0 || uint64(native.Dev) != record.Device || native.Ino != record.Inode {
		return errors.Join(ErrBinding, err, fsErr)
	}
	var probe [1]byte
	n, err := file.ReadAt(probe[:], 0)
	if n != 0 || err != io.EOF {
		return errors.Join(ErrBinding, err)
	}
	return errors.Join(observeInitialFile(own), files.local())
}

func (s *recoveryInitialReloadAttempt) openUnitContinuation() error {
	u := s.unit
	if u == nil || u.original == nil || u.mutation != nil || s.original.cut.Boundary.Records != 116 || s.proof.unitBirth != &u.birth {
		return ErrBinding
	}
	parent := s.files.files.dirs[filepath.Dir(u.original.path)]
	if parent == nil {
		return ErrBinding
	}
	if err := s.observe(); err != nil {
		return err
	}
	var err error
	u.mutation, err = fixedfile.ContinueBirth(s.ctx, u.original.path, parent.info, u.original.info, s.candidate.files["rendered-forwarding.service"], 0644, 0)
	if err != nil {
		return err
	}
	if !sameInputFile(u.mutation.Identity(), u.original.info) {
		return ErrBinding
	}
	return s.observe()
}

func (s *recoveryInitialReloadAttempt) continueUnit() error {
	u := s.unit
	if u != nil && s.original.cut.Boundary.Records <= 115 {
		if err := s.createUnit(); err != nil {
			return err
		}
	}
	if u == nil || u.mutation == nil || u.ready || s.publication.births == nil {
		return ErrBinding
	}
	if err := s.observe(); err != nil {
		return err
	}
	if s.original.cut.Boundary.Records == 116 {
		if err := s.publication.births.record("continued-file-retained", u.original.path, u.original.info, digest(nil), 0600, 0, 0); err != nil {
			return err
		}
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := u.commit(s.candidate.files["rendered-forwarding.service"], 0644); err != nil {
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

// The concrete Initial file continuations use the same physical Commit and
// independent readback. Their admitting actors own path, phase and journal.
func (u *initialFileContinuation) commit(wanted []byte, mode os.FileMode) error {
	committed, err := u.mutation.Commit()
	if err != nil {
		return err
	}
	current, err := u.original.file.Stat()
	named, nameErr := os.Lstat(u.original.path)
	if err != nil || nameErr != nil || !sameInputFile(committed, current) || !sameInputFile(committed, named) || !nodeAccess(current, mode, 0, 0, false) {
		return errors.Join(ErrBinding, err, nameErr)
	}
	actual := make([]byte, len(wanted)+1)
	n, readErr := u.original.file.ReadAt(actual, 0)
	if readErr != io.EOF || n != len(wanted) || !bytes.Equal(actual[:n], wanted) {
		clear(actual)
		return errors.Join(ErrBinding, readErr)
	}
	clear(actual)
	u.original.info = committed
	u.original.body = bytes.Clone(wanted)
	u.ready = true
	return observeInitialFile(u.original)
}

func (u *initialFileContinuation) close() error {
	if u == nil {
		return nil
	}
	return u.mutation.Close()
}
