package installation

import (
	"bytes"
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"os"
	"path/filepath"
	"syscall"
)

// Root-controlled presence alone does not identify a directory created by this
// transaction. Retain the original inode and intended access before mutation;
// explicit recovery must validate these bytes rather than adopt a pathname.
type fixedDirectoryCreation struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	Path             string `json:"path"`
	Device           uint64 `json:"device"`
	Inode            uint64 `json:"inode"`
	PreviousMode     uint32 `json:"previous_mode"`
	PreviousGID      uint32 `json:"previous_gid"`
	Mode             uint32 `json:"mode"`
	GID              uint32 `json:"gid"`
}

func (stage *installationTransaction) recordFixedDirectory(ctx context.Context, directory string, original os.FileInfo, mode os.FileMode, gid uint32, promotion bool) error {
	if stage == nil || stage.journal == nil || original == nil || !canonicalPath(directory) || directory == "/" ||
		(mode != 0700 && mode != 0755 && mode != 0555 && mode != 0710) ||
		((mode == 0710) != (gid != 0)) {
		return ErrInput
	}
	if len(stage.journal.Bytes(journal.Transitions, "0003.json")) == 0 {
		return ErrBinding
	}
	native, ok := original.Sys().(*syscall.Stat_t)
	if !ok || !privateJournalDirectory(original) || native.Ino == 0 || native.Dev == 0 {
		return ErrBinding
	}
	if promotion && mode != 0555 {
		return ErrBinding
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if err := stage.journal.Ensure(ctx, journal.DirectoryCreations); err != nil {
		return err
	}
	record := fixedDirectoryCreation{Schema: "ardents-endpoint-directory-creation-v1", GenerationDigest: stage.selected.GenerationDigest,
		Path: directory, Device: uint64(native.Dev), Inode: native.Ino, PreviousMode: uint32(original.Mode().Perm()), PreviousGID: native.Gid, Mode: uint32(mode), GID: gid}
	body, err := canonicalJSON(record)
	if err != nil {
		return err
	}
	name := digestHex([]byte(directory))
	if promotion {
		name += "-access"
	}
	return stage.journal.Write(ctx, journal.DirectoryCreations, name+".json", body)
}

func (stage *installationTransaction) fixedPhase(ctx context.Context, name, phase string) error {
	if name == "0003.json" && phase == "installing-fixed-resources" {
		if len(stage.journal.Bytes(journal.Transitions, "0002.json")) == 0 {
			return ErrBinding
		}
	} else if name == "0004.json" && phase == "fixed-resources-installed" {
		if len(stage.journal.Bytes(journal.Transitions, "0003.json")) == 0 {
			return ErrBinding
		}
	} else if name == "0005.json" && phase == "publishing-selection" {
		if len(stage.journal.Bytes(journal.Transitions, "0004.json")) == 0 {
			return ErrBinding
		}
	} else if name == "0006.json" && phase == "reloading-manager" {
		if len(stage.journal.Bytes(journal.Transitions, "0005.json")) == 0 {
			return ErrBinding
		}
		if _, selected := stage.fixed[filepath.Join(stage.lease.path, "selection.json")]; !selected {
			return ErrBinding
		}
	} else if name == "0007.json" && phase == "installed-stopped" {
		if len(stage.journal.Bytes(journal.Transitions, "0006.json")) == 0 {
			return ErrBinding
		}
	} else {
		return ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	body, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: phase})
	if err != nil {
		return err
	}
	return stage.journal.Write(ctx, journal.Transitions, name, body)
}

func (stage *installationTransaction) ensureRootParent(ctx context.Context, directory string) error {
	if _, err := os.Lstat(directory); os.IsNotExist(err) {
		return stage.birthFixedDirectory(ctx, directory, 0755, 0)
	}
	_, err := rootDirectoryAncestors(directory)
	return err
}

func (stage *installationTransaction) birthFixedDirectory(ctx context.Context, directory string, mode os.FileMode, gid uint32) (returnedErr error) {
	if stage == nil || stage.journal == nil || !canonicalPath(directory) || directory == "/" ||
		(mode != 0700 && mode != 0755 && mode != 0555 && mode != 0710) || ((mode == 0710) != (gid != 0)) {
		return ErrInput
	}
	if len(stage.journal.Bytes(journal.Transitions, "0003.json")) == 0 {
		return ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if err := requireAbsentManagedPath(directory); err != nil {
		return err
	}
	if err := stage.ensureRootParent(ctx, filepath.Dir(directory)); err != nil {
		return err
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(directory, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	birth, err := file.Stat()
	pathBirth, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !privateJournalDirectory(birth) || !os.SameFile(birth, pathBirth) {
		return errors.Join(ErrBinding, err, pathErr)
	}
	if err := errors.Join(file.Sync(), syncDirectDirectory(filepath.Dir(directory))); err != nil {
		return err
	}
	if err := stage.recordFixedDirectory(ctx, directory, birth, mode, gid, false); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := file.Stat()
	currentPath, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !sameObservedDirectory(birth, current) || !sameObservedDirectory(birth, currentPath) {
		return errors.Join(ErrBinding, err, pathErr)
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
	pathInfo, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !os.SameFile(birth, info) || !sameObservedDirectory(info, pathInfo) || info.Mode() != os.ModeDir|mode {
		return errors.Join(ErrBinding, err, pathErr)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != gid {
		return ErrBinding
	}
	if stage.fixedDirectories == nil {
		stage.fixedDirectories = make(map[string]os.FileInfo)
	}
	stage.fixedDirectories[directory] = info
	return errors.Join(syncDirectDirectory(filepath.Dir(directory)), ctx.Err())
}

func (stage *installationTransaction) changeFixedDirectoryMode(ctx context.Context, directory string, mode os.FileMode) (returnedErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	original := stage.fixedDirectories[directory]
	file, err := os.OpenFile(directory, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	before, err := file.Stat()
	if err != nil || !sameObservedDirectory(original, before) {
		return errors.Join(ErrBinding, err)
	}
	native, ok := before.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrBinding
	}
	if err := stage.recordFixedDirectory(ctx, directory, before, mode, native.Gid, true); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := file.Stat()
	currentPath, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !sameObservedDirectory(before, current) || !sameObservedDirectory(before, currentPath) {
		return errors.Join(ErrBinding, err, pathErr)
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	info, err := file.Stat()
	pathInfo, pathErr := os.Lstat(directory)
	if err != nil || pathErr != nil || !os.SameFile(original, info) || !sameObservedDirectory(info, pathInfo) || info.Mode() != os.ModeDir|mode {
		return errors.Join(ErrBinding, err, pathErr)
	}
	stage.fixedDirectories[directory] = info
	// The owned parent changed access on this exact inode. Its already pinned
	// children keep their original file identities and bytes, but must observe
	// the new parent mode rather than treating our promotion as substitution.
	for filename, observation := range stage.fixed {
		if filepath.Dir(filename) == directory {
			if !sameObservedDirectory(original, observation.parent) {
				return ErrBinding
			}
			observation.parent = info
			stage.fixed[filename] = observation
		}
	}
	return errors.Join(stage.observe(), ctx.Err())
}

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
	file   fileObservation
}

// The transaction must observe manager absence and its original account/roots
// before selecting fixed effects. This mechanism itself grants no admission.
// Every failure retains the born leaf and journal for explicit repair.
func (stage *installationTransaction) createFixedFile(ctx context.Context, filename string, body []byte, mode os.FileMode, gid uint32) (returnedErr error) {
	if stage == nil || stage.journal == nil || ctx == nil || !canonicalPath(filename) || filename == "/" || len(body) == 0 || len(body) > 64<<20 ||
		(mode != 0644 && mode != 0555 && mode != 0640) {
		return ErrInput
	}
	if len(stage.journal.Bytes(journal.Transitions, "0003.json")) == 0 {
		return ErrBinding
	}
	isSelection := filename == filepath.Join(stage.lease.path, "selection.json")
	if isSelection {
		wanted, err := canonicalJSON(stage.selected)
		if len(stage.journal.Bytes(journal.Transitions, "0005.json")) == 0 || err != nil || !bytes.Equal(body, wanted) || mode != 0640 {
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
	if err := stage.journal.Ensure(ctx, journal.Creations); err != nil {
		return err
	}
	parentInfo, err := os.Lstat(filepath.Dir(filename))
	trustedParent := rootDirectory(parentInfo)
	if isSelection {
		trustedParent = sameObservedDirectory(stage.lease.identity, parentInfo)
	}
	if err != nil || !trustedParent {
		return errors.Join(ErrBinding, err)
	}
	frozen := bytes.Clone(body)
	mutation, err := fixedfile.Create(ctx, filename, parentInfo, frozen, mode, gid)
	if mutation != nil {
		defer func() { returnedErr = errors.Join(returnedErr, fixedResourceError(mutation.Close())) }()
	}
	if err != nil {
		return fixedResourceError(err)
	}
	birth := mutation.Identity()
	native, ok := birth.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrBinding
	}
	record := fixedCreationRecord{Schema: "ardents-endpoint-installation-creation-v1", GenerationDigest: stage.selected.GenerationDigest,
		Path: filename, Device: uint64(native.Dev), Inode: native.Ino, Mode: uint32(mode), GID: gid, CandidateDigest: digestHex(frozen)}
	recordBody, err := canonicalJSON(record)
	if err != nil || len(recordBody) > 64<<10 {
		return errors.Join(ErrBinding, err)
	}
	if err := stage.journal.Write(ctx, journal.Creations, digestHex([]byte(filename))+".json", recordBody); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	// The physical owner rechecks the original empty inode after journal I/O.
	info, err := mutation.Commit()
	if info != nil {
		if stage.fixed == nil {
			stage.fixed = make(map[string]fixedFileObservation)
		}
		stage.fixed[filename] = fixedFileObservation{parent: parentInfo, file: fileObservation{identity: info, body: frozen, mode: mode, gid: gid}}
	}
	return fixedResourceError(err)
}

// Physical failures retain their native causes and the domain's error classes.
func fixedResourceError(err error) error {
	if errors.Is(err, fixedfile.ErrBinding) {
		return errors.Join(ErrBinding, err)
	}
	if errors.Is(err, fixedfile.ErrInput) {
		return errors.Join(ErrInput, err)
	}
	return err
}

func observeFixedFile(filename string, expected fixedFileObservation) (returnedErr error) {
	parentInfo, err := os.Lstat(filepath.Dir(filename))
	if err != nil || !sameObservedDirectory(expected.parent, parentInfo) {
		return errors.Join(ErrBinding, err)
	}
	root, err := os.OpenRoot(filepath.Dir(filename))
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, root.Close()) }()
	handle, err := root.Stat(".")
	if err != nil || !sameObservedDirectory(expected.parent, handle) {
		return errors.Join(ErrBinding, err)
	}
	if err := observeStagedFile(root, filepath.Base(filename), expected.file); err != nil {
		return err
	}
	after, err := os.Lstat(filepath.Dir(filename))
	if err != nil || !sameObservedDirectory(expected.parent, after) {
		return errors.Join(ErrBinding, err)
	}
	return nil
}
