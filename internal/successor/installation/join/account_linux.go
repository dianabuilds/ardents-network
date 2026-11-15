package join

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/nodeprincipal"
)

type accountFile struct {
	path string
	file *os.File
	info os.FileInfo
	raw  []byte
}

// nodeAccount ties a role's initial transaction to one independent native
// creator. The creator holds its own files; this role owns admission and journal.
type nodeAccount struct {
	original *nodeprincipal.Creator
	uid, gid uint32
}

func prepareNodeAccount(ctx context.Context, inputs Inputs) (*nodeAccount, error) {
	original, err := nodeprincipal.PrepareCreator(ctx)
	a := &nodeAccount{original: original}
	if err != nil {
		return a, nodeprincipal.MapObservationError(err, ErrInput, ErrBinding)
	}
	a.uid, a.gid = original.ID()
	return a, errors.Join(inputs.Check(), a.checkBefore(), a.checkAbsentNSS())
}

func (a *nodeAccount) checkBefore() error {
	return nodeprincipal.MapObservationError(a.original.CheckBefore(), ErrInput, ErrBinding)
}
func (a *nodeAccount) checkAbsentNSS() error {
	return nodeprincipal.MapObservationError(a.original.CheckAbsentNSS(), ErrInput, ErrBinding)
}
func (a *nodeAccount) check() error {
	return nodeprincipal.MapObservationError(a.original.Check(), ErrInput, ErrBinding)
}
func (a *nodeAccount) close() error {
	if a == nil {
		return nil
	}
	return nodeprincipal.MapObservationError(a.original.Close(), ErrInput, ErrBinding)
}
func nodeAccountAuxiliaryPaths() []string { return nodeprincipal.AuxiliaryPaths() }

// The role keeps the original admission and journal order around the physical
// account owner. Detached records cannot create or adopt an account.
func (a *nodeAccount) create(j *initialBirths, inputs Inputs, admit, admitCreated func() error) error {
	if err := errors.Join(a.checkBefore(), inputs.Check()); err != nil {
		return err
	}
	records, err := a.original.Preimages()
	if err != nil {
		return nodeprincipal.MapObservationError(err, ErrInput, ErrBinding)
	}
	for i, record := range records {
		phase := "account-preimage"
		if i >= 4 {
			phase = "account-auxiliary-preimage"
		}
		if err := j.record(phase, record.Path, record.Info, record.SHA256, record.Mode, record.UID, record.GID); err != nil {
			return err
		}
	}
	if err := j.record("account-intent", "ardents-node", nil, "", 0, a.uid, a.gid); err != nil {
		return err
	}
	if err := errors.Join(a.checkBefore(), inputs.Check()); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	if err := a.checkAbsentNSS(); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	if err := nodeprincipal.MapObservationError(a.original.Execute(), ErrInput, ErrBinding); err != nil {
		return err
	}
	for _, path := range append([]string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"}, nodeAccountAuxiliaryPaths()...) {
		if err := admitCreated(); err != nil {
			return err
		}
		record, phase, present, err := a.original.SyncResult(path)
		if err != nil {
			return nodeprincipal.MapObservationError(err, ErrInput, ErrBinding)
		}
		if !present {
			if err := j.record(phase, path, nil, "", 0, 0, 0); err != nil {
				return err
			}
			continue
		}
		if err := admitCreated(); err != nil {
			return err
		}
		if err := j.record(phase, path, record.Info, record.SHA256, record.Mode, record.UID, record.GID); err != nil {
			return err
		}
	}
	if err := admitCreated(); err != nil {
		return err
	}
	return j.record("account-ready", "ardents-node", nil, "", 0, a.uid, a.gid)
}
func readNodeAccountFile(path string) (result *accountFile, err error) {
	// Input paths are a closed literal inventory, never caller-selected helpers.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	result = &accountFile{path: path, file: f}
	defer func() {
		if err != nil {
			err = errors.Join(err, f.Close())
			clear(result.raw)
			result = nil
		}
	}()
	result.info, err = f.Stat()
	if err != nil {
		return result, err
	}
	st, ok := result.info.Sys().(*syscall.Stat_t)
	if !ok || !result.info.Mode().IsRegular() || st.Uid != 0 || st.Nlink != 1 || result.info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || result.info.Mode().Perm()&0022 != 0 || result.info.Size() > 8<<20 {
		return result, ErrBinding
	}
	result.raw, err = io.ReadAll(io.LimitReader(f, 8<<20+1))
	if err != nil || int64(len(result.raw)) != result.info.Size() {
		return result, errors.Join(ErrBinding, err)
	}
	return result, checkNodeAccountFile(result)
}

func checkNodeAccountFile(f *accountFile) error {
	info, err := f.file.Stat()
	named, ne := os.Lstat(f.path)
	if err != nil || ne != nil || !sameInputFile(f.info, info) || !sameInputFile(f.info, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	if _, err = f.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(f.file, int64(len(f.raw))+1))
	equal := bytes.Equal(raw, f.raw)
	clear(raw)
	if err != nil || !equal {
		return errors.Join(ErrBinding, err)
	}
	info, err = f.file.Stat()
	named, ne = os.Lstat(f.path)
	if err != nil || ne != nil || !sameInputFile(f.info, info) || !sameInputFile(f.info, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	return nil
}
