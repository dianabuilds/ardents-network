package installation

import (
	"bytes"
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/generation"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Retain all borrowed directory handles until staging and its failure record
// physically finish. The Installation owner closes this before returning lease.
type installationTransaction struct {
	lease            *installationLease
	directories      []*installationContainer
	journal          *journal.Transition
	generation       *generation.Owner
	sealed           *generation.Snapshot
	prefix           *generation.Prefix
	sealedIdentity   os.FileInfo
	fixed            map[string]fixedFileObservation
	fixedDirectories map[string]os.FileInfo
	selected         generationSelection
	intent           stagedFile
	archivedIntent   bool
	terminal         error
	parents          []*stagingParent
	intentAttempted  bool
	barrier          *installedStartBarrier
}

type installationContainer struct {
	parent, root *os.Root
	file         *os.File
	name         string
	identity     os.FileInfo
	generations  map[string]*generation.Owner
	journals     map[string]*journal.Transition
}

type stagedFile struct {
	identity os.FileInfo
	body     []byte
	gid      uint32
	mode     os.FileMode
}

func (stage *installationTransaction) createContainer(ctx context.Context, parent *os.Root, name string) (*installationContainer, error) {
	if ctx == nil || stage == nil || stage.lease == nil || parent != stage.lease.root || (name != "journals" && name != "generations") {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := stage.lease.observe(); err != nil {
		return nil, err
	}
	if err := parent.Mkdir(name, 0700); err != nil {
		return nil, err
	}
	file, err := parent.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	dir := &installationContainer{parent: parent, file: file, name: name}
	stage.directories = append(stage.directories, dir)
	dir.identity, err = file.Stat()
	if err != nil || !privateJournalDirectory(dir.identity) {
		return nil, errors.Join(ErrBinding, err)
	}
	dir.root, err = parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	if err := dir.observe(); err != nil {
		return nil, err
	}
	return dir, errors.Join(file.Sync(), syncStagingRoot(parent))
}

func (stage *installationTransaction) observe() (returnedErr error) {
	if stage.terminal != nil {
		return stage.terminal
	}
	defer func() {
		if returnedErr != nil {
			stage.terminal = returnedErr
		}
	}()
	if err := stage.lease.observe(); err != nil {
		return err
	}
	if err := stage.observeIntent(); err != nil {
		return err
	}
	for _, parent := range stage.parents {
		if err := parent.observe(); err != nil {
			return err
		}
	}
	for _, dir := range stage.directories {
		if err := dir.observe(); err != nil {
			return err
		}
	}
	if stage.generation != nil {
		if err := stage.generation.Observe(); err != nil {
			return errors.Join(ErrBinding, err)
		}
	}
	if stage.prefix != nil {
		if err := stage.prefix.Observe(); err != nil {
			return errors.Join(ErrBinding, err)
		}
	}
	if stage.sealed != nil {
		if err := stage.sealed.Observe(); err != nil {
			return errors.Join(ErrBinding, err)
		}
	}
	if stage.journal != nil {
		if err := stage.journal.Observe(); err != nil {
			return errors.Join(ErrBinding, err)
		}
	}
	for filename, expected := range stage.fixed {
		if err := observeFixedFile(filename, expected); err != nil {
			return err
		}
	}
	for directory, expected := range stage.fixedDirectories {
		info, err := os.Lstat(directory)
		if err != nil || !sameStagingDirectory(expected, info) {
			return errors.Join(ErrBinding, err)
		}
	}
	if stage.barrier != nil {
		return stage.barrier.observe()
	}
	return nil
}

func (dir *installationContainer) observe() error {
	if dir.root == nil || dir.file == nil || dir.identity == nil {
		return ErrBinding
	}
	pathInfo, pathErr := dir.parent.Lstat(dir.name)
	rootInfo, rootErr := dir.root.Stat(".")
	fdInfo, fdErr := dir.file.Stat()
	for _, info := range []os.FileInfo{pathInfo, rootInfo, fdInfo} {
		if !sameStagingDirectory(dir.identity, info) {
			return errors.Join(ErrBinding, pathErr, rootErr, fdErr)
		}
	}
	reader, err := dir.root.Open(".")
	if err != nil {
		return err
	}
	names, readErr := reader.Readdirnames(len(dir.generations) + len(dir.journals) + 1)
	closeErr := reader.Close()
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	if err := errors.Join(closeErr, readErr); err != nil {
		return err
	}
	if len(names) != len(dir.generations)+len(dir.journals) {
		return ErrBinding
	}
	for _, name := range names {
		if log, own := dir.journals[name]; own {
			if err := log.Observe(); err != nil {
				return errors.Join(ErrBinding, err)
			}
		} else if artifact, own := dir.generations[name]; own {
			if err := artifact.Observe(); err != nil {
				return errors.Join(ErrBinding, err)
			}
		} else {
			return ErrBinding
		}
	}
	return nil
}

func sameStagingDirectory(original, current os.FileInfo) bool {
	if original == nil || current == nil || !current.IsDir() || !os.SameFile(original, current) || original.Mode() != current.Mode() {
		return false
	}
	a, aOK := original.Sys().(*syscall.Stat_t)
	b, bOK := current.Sys().(*syscall.Stat_t)
	return aOK && bOK && a.Uid == 0 && b.Uid == 0 && a.Gid == b.Gid && current.Mode().Perm()&0022 == 0
}

func writeStagedFile(ctx context.Context, root *os.Root, name string, body []byte, mode os.FileMode, gid uint32) (result stagedFile, returnedErr error) {
	if err := ctx.Err(); err != nil {
		return stagedFile{}, err
	}
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return stagedFile{}, err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	birth, err := file.Stat()
	if err != nil || !ownedRequestFile(birth) || birth.Mode().Perm() != 0600 || birth.Size() != 0 {
		return stagedFile{}, errors.Join(ErrBinding, err)
	}
	written, err := file.Write(body)
	if err != nil || written != len(body) {
		return stagedFile{}, errors.Join(io.ErrShortWrite, err)
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return stagedFile{}, err
	}
	if err := file.Chmod(mode); err != nil {
		return stagedFile{}, err
	}
	if err := file.Sync(); err != nil {
		return stagedFile{}, err
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(birth, info) {
		return stagedFile{}, errors.Join(ErrBinding, err)
	}
	result = stagedFile{identity: info, body: bytes.Clone(body), gid: gid, mode: mode}
	return result, errors.Join(observeStagedFile(root, name, result), ctx.Err())
}

func observeStagedFile(root *os.Root, name string, expected stagedFile) (returnedErr error) {
	before, err := root.Lstat(name)
	if err != nil || !stagedFileMatches(expected, before) {
		return errors.Join(ErrBinding, err)
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil || !stagedFileMatches(expected, info) {
		return errors.Join(ErrBinding, err)
	}
	body, err := io.ReadAll(io.LimitReader(file, int64(len(expected.body))+1))
	if err != nil || !bytes.Equal(body, expected.body) {
		return errors.Join(ErrBinding, err)
	}
	final, err := root.Lstat(name)
	finalHandle, handleErr := file.Stat()
	if err != nil || handleErr != nil || !stagedFileMatches(expected, final) || !stagedFileMatches(expected, finalHandle) {
		return errors.Join(ErrBinding, err, handleErr)
	}
	return nil
}

func stagedFileMatches(expected stagedFile, info os.FileInfo) bool {
	if expected.identity == nil || info == nil || !info.Mode().IsRegular() || !os.SameFile(expected.identity, info) ||
		info.Mode() != expected.mode || info.Size() != int64(len(expected.body)) || !info.ModTime().Equal(expected.identity.ModTime()) {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == expected.gid && native.Nlink == 1
}

func syncStagingRoot(root *os.Root) (returnedErr error) {
	file, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	return file.Sync()
}

func (stage *installationTransaction) close() error {
	if stage.barrier != nil {
		stage.terminal = errors.Join(stage.terminal, stage.barrier.close())
	}
	stage.terminal = errors.Join(stage.terminal, stage.generation.Close(), stage.journal.Close())
	stage.terminal = errors.Join(stage.terminal, stage.sealed.Close(), stage.prefix.Close())
	for i := len(stage.directories) - 1; i >= 0; i-- {
		dir := stage.directories[i]
		if dir.root != nil {
			stage.terminal = errors.Join(stage.terminal, dir.root.Close())
			dir.root = nil
		}
		if dir.file != nil {
			stage.terminal = errors.Join(stage.terminal, dir.file.Close())
			dir.file = nil
		}
	}
	for index := len(stage.parents) - 1; index >= 0; index-- {
		stage.terminal = errors.Join(stage.terminal, stage.parents[index].close())
	}
	return stage.terminal
}

// Create generation custody under the original transaction, with independent
// native handles. The owning operation records its birth before the first Write.
func (stage *installationTransaction) createGeneration(ctx context.Context, parent *os.Root, gid uint32) (*generation.Owner, error) {
	if err := stage.observe(); err != nil {
		return nil, err
	}
	expected, err := parent.Stat(".")
	if err != nil {
		return nil, err
	}
	owned, err := generation.Create(ctx, filepath.Join(stage.lease.path, "generations"), expected, stage.selected.GenerationDigest, gid)
	if owned != nil {
		for _, directory := range stage.directories {
			if directory.root == parent {
				if directory.generations == nil {
					directory.generations = make(map[string]*generation.Owner)
				}
				directory.generations[stage.selected.GenerationDigest] = owned
			}
		}
	}
	return owned, err
}

// Transition custody has independent descriptors under the original journals
// container. Installation keeps admission and closes it only after original join.
func (stage *installationTransaction) createTransition(ctx context.Context, parent *os.Root) (*journal.Transition, error) {
	if err := stage.observe(); err != nil {
		return nil, err
	}
	expected, err := parent.Stat(".")
	if err != nil {
		return nil, err
	}
	owned, err := journal.CreateTransition(ctx, filepath.Join(stage.lease.path, "journals"), expected, stage.selected.GenerationDigest)
	if owned != nil {
		for _, directory := range stage.directories {
			if directory.root == parent {
				if directory.journals == nil {
					directory.journals = make(map[string]*journal.Transition)
				}
				directory.journals[stage.selected.GenerationDigest] = owned
			}
		}
	}
	return owned, err
}

// Existing containers lend directory identity, not ownership of all historical
// contents. Fresh staging directories separately retain their exact inventory.
type stagingParent struct {
	parent, root *os.Root
	file         *os.File
	name         string
	identity     os.FileInfo
}

func (stage *installationTransaction) borrowParent(ctx context.Context, name string, gid uint32, expected os.FileInfo) (result *stagingParent, returnedErr error) {
	if ctx == nil || stage == nil || stage.lease == nil || gid == 0 || (name != "journals" && name != "generations") {
		return nil, ErrInput
	}
	if err := errors.Join(ctx.Err(), stage.lease.observe()); err != nil {
		return nil, err
	}
	before, err := stage.lease.root.Lstat(name)
	if err != nil || before == nil {
		return nil, errors.Join(ErrBinding, err)
	}
	if name == "journals" {
		if !privateJournalDirectory(before) {
			return nil, ErrBinding
		}
	} else {
		native, ok := before.Sys().(*syscall.Stat_t)
		if !sameStagingDirectory(expected, before) || !ok || native.Uid != 0 || native.Gid != gid || before.Mode() != os.ModeDir|0750 {
			return nil, ErrBinding
		}
	}
	borrowed := &stagingParent{parent: stage.lease.root, name: name, identity: before}
	defer func() {
		returnedErr = errors.Join(returnedErr, ctx.Err())
		if returnedErr != nil {
			returnedErr = errors.Join(returnedErr, borrowed.close())
			result = nil
		}
	}()
	borrowed.file, err = stage.lease.root.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	borrowed.root, err = stage.lease.root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	if err := borrowed.observe(); err != nil {
		return nil, err
	}
	stage.parents = append(stage.parents, borrowed)
	return borrowed, nil
}

func (parent *stagingParent) observe() error {
	if parent == nil || parent.parent == nil || parent.root == nil || parent.file == nil || parent.identity == nil {
		return ErrBinding
	}
	pathInfo, pathErr := parent.parent.Lstat(parent.name)
	rootInfo, rootErr := parent.root.Stat(".")
	fileInfo, fileErr := parent.file.Stat()
	for _, info := range []os.FileInfo{pathInfo, rootInfo, fileInfo} {
		if !sameStagingDirectory(parent.identity, info) {
			return errors.Join(ErrBinding, pathErr, rootErr, fileErr)
		}
	}
	return nil
}

func (parent *stagingParent) close() error {
	if parent == nil {
		return nil
	}
	var returnedErr error
	if parent.root != nil {
		returnedErr = errors.Join(returnedErr, parent.root.Close())
		parent.root = nil
	}
	if parent.file != nil {
		returnedErr = errors.Join(returnedErr, parent.file.Close())
		parent.file = nil
	}
	return returnedErr
}
