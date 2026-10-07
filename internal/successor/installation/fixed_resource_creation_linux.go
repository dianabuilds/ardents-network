package installation

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Preserve creation-v1 separately from successor truncation records. A birth
// recorded here owns precisely one inode, never another file at the same path.
type fixedCreationRecord struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	Path             string `json:"path"`
	Device           uint64 `json:"device"`
	Inode            uint64 `json:"inode"`
	Mode             uint32 `json:"mode"`
	GID              uint32 `json:"gid"`
	PreviousDigest   string `json:"previous_digest"`
	CandidateDigest  string `json:"candidate_digest"`
}

type fixedFileObservation struct {
	parent os.FileInfo
	file   stagedFile
}

// The transaction must observe manager absence and its original account/roots
// before selecting fixed effects. This mechanism itself grants no admission.
// Every failure retains the born leaf and journal for explicit repair.
func (stage *generationStage) createFixedFile(ctx context.Context, filename string, body []byte, mode os.FileMode, gid uint32) (returnedErr error) {
	if stage == nil || stage.journal == nil || ctx == nil || !canonicalPath(filename) || filename == "/" || len(body) == 0 || len(body) > 64<<20 ||
		(mode != 0644 && mode != 0555 && mode != 0640) {
		return ErrInput
	}
	if _, intent := stage.journal.files["0003.json"]; !intent {
		return ErrBinding
	}
	isSelection := filename == filepath.Join(stage.lease.path, "selection.json")
	if isSelection {
		wanted, err := canonicalJSON(stage.selected)
		if _, intent := stage.journal.files["0005.json"]; !intent || err != nil || !bytes.Equal(body, wanted) || mode != 0640 {
			return errors.Join(ErrBinding, err)
		}
		native, ok := stage.lease.identity.Sys().(*syscall.Stat_t)
		if !ok || native.Gid != gid || gid == 0 || stage.lease.identity.Mode() != os.ModeDir|0750 {
			return ErrBinding
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if !isSelection {
		if _, err := rootDirectoryAncestors(filepath.Dir(filename)); err != nil {
			return err
		}
	}
	if stage.creations == nil {
		created, err := stage.birth(ctx, stage.journal.root, "creations")
		if err != nil {
			return err
		}
		stage.creations = created
	}
	parent, err := os.OpenRoot(filepath.Dir(filename))
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, parent.Close()) }()
	parentInfo, err := parent.Stat(".")
	pathParent, pathErr := os.Lstat(filepath.Dir(filename))
	trustedParent := rootDirectory(parentInfo)
	if isSelection {
		trustedParent = sameStagingDirectory(stage.lease.identity, parentInfo)
	}
	if err != nil || pathErr != nil || !trustedParent || !os.SameFile(parentInfo, pathParent) {
		return errors.Join(ErrBinding, err, pathErr)
	}
	name := filepath.Base(filename)
	file, err := parent.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	birth, err := file.Stat()
	pathBirth, pathErr := parent.Lstat(name)
	if err != nil || pathErr != nil || !ownedRequestFile(birth) || birth.Mode().Perm() != 0600 || birth.Size() != 0 || !sameRequestFile(birth, pathBirth) {
		return errors.Join(ErrBinding, err, pathErr)
	}
	if err := errors.Join(file.Sync(), syncStagingRoot(parent)); err != nil {
		return err
	}
	native := birth.Sys().(*syscall.Stat_t)
	frozen := bytes.Clone(body)
	record := fixedCreationRecord{Schema: "ardents-endpoint-installation-creation-v1", GenerationDigest: stage.selected.GenerationDigest,
		Path: filename, Device: uint64(native.Dev), Inode: native.Ino, Mode: uint32(mode), GID: gid, CandidateDigest: digestHex(frozen)}
	recordBody, err := canonicalJSON(record)
	if err != nil || len(recordBody) > 64<<10 {
		return errors.Join(ErrBinding, err)
	}
	if err := stage.creations.write(ctx, digestHex([]byte(filename))+".json", recordBody, 0600, 0); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	// Reobserve the original empty inode immediately before writing its payload.
	current, err := file.Stat()
	currentPath, pathErr := parent.Lstat(name)
	if err != nil || pathErr != nil || !sameRequestFile(birth, current) || !sameRequestFile(birth, currentPath) {
		return errors.Join(ErrBinding, err, pathErr)
	}
	written, err := file.Write(frozen)
	if err != nil || written != len(frozen) {
		return errors.Join(io.ErrShortWrite, err)
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(birth, info) {
		return errors.Join(ErrBinding, err)
	}
	observation := fixedFileObservation{parent: parentInfo, file: stagedFile{identity: info, body: frozen, mode: mode, gid: gid}}
	if stage.fixed == nil {
		stage.fixed = make(map[string]fixedFileObservation)
	}
	stage.fixed[filename] = observation
	if err := errors.Join(observeStagedFile(parent, name, observation.file), syncStagingRoot(parent)); err != nil {
		return err
	}
	afterParent, err := os.Lstat(filepath.Dir(filename))
	if err != nil || !sameStagingDirectory(parentInfo, afterParent) {
		return errors.Join(ErrBinding, err)
	}
	return ctx.Err()
}

func observeFixedFile(filename string, expected fixedFileObservation) (returnedErr error) {
	parentInfo, err := os.Lstat(filepath.Dir(filename))
	if err != nil || !sameStagingDirectory(expected.parent, parentInfo) {
		return errors.Join(ErrBinding, err)
	}
	root, err := os.OpenRoot(filepath.Dir(filename))
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, root.Close()) }()
	handle, err := root.Stat(".")
	if err != nil || !sameStagingDirectory(expected.parent, handle) {
		return errors.Join(ErrBinding, err)
	}
	if err := observeStagedFile(root, filepath.Base(filename), expected.file); err != nil {
		return err
	}
	after, err := os.Lstat(filepath.Dir(filename))
	if err != nil || !sameStagingDirectory(expected.parent, after) {
		return errors.Join(ErrBinding, err)
	}
	return nil
}
