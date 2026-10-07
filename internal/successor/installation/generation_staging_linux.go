package installation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"
)

type generationTransition struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	BindingDigest    string `json:"binding_digest"`
	Phase            string `json:"phase"`
	OriginalError    string `json:"original_error,omitempty"`
}

type generationBirth struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	Device           uint64 `json:"device"`
	Inode            uint64 `json:"inode"`
}

type initialTransitionIntent struct {
	Schema           string              `json:"schema"`
	Previous         generationSelection `json:"previous"`
	Candidate        generationSelection `json:"candidate"`
	CandidateBinding generationBinding   `json:"candidate_binding"`
	Request          installationRequest `json:"request"`
}

// Retain all borrowed directory handles until staging and its failure record
// physically finish. The Installation owner closes this before returning lease.
type generationStage struct {
	lease               *installationLease
	directories         []*stagingDirectory
	journal, generation *stagingDirectory
	creations           *stagingDirectory
	fixed               map[string]fixedFileObservation
	fixedDirectories    map[string]os.FileInfo
	selected            generationSelection
	intent              stagedFile
	archivedIntent      *stagedFile
	terminal            error
}

type stagingDirectory struct {
	parent, root *os.Root
	file         *os.File
	name         string
	identity     os.FileInfo
	files        map[string]stagedFile
	children     map[string]*stagingDirectory
}

type stagedFile struct {
	identity os.FileInfo
	body     []byte
	gid      uint32
	mode     os.FileMode
}

// Only the genuine initial-preparation owner calls this after assembling the
// exact authorized bytes and observing actual account/root provenance.
func stageInitialGeneration(ctx context.Context, lease *installationLease, request Request, files map[string][]byte, selected generationSelection, gid uint32) (result *generationStage, returnedErr error) {
	if ctx == nil || request.declared == nil || lease == nil || gid == 0 ||
		selected.Schema != "ardents-endpoint-installation-selection-v1" || !canonicalDigest(selected.GenerationDigest) ||
		!canonicalDigest(selected.BindingDigest) || selected.BindingDigest != digestHex(files["binding.json"]) {
		return nil, ErrBinding
	}
	var binding generationBinding
	if err := json.Unmarshal(files["binding.json"], &binding); err != nil {
		return nil, err
	}
	body, err := canonicalJSON(binding)
	if err != nil || !bytes.Equal(body, files["binding.json"]) || binding.InstallationRoot != lease.path ||
		request.declared.InstallationRoot != lease.path || binding.GenerationDigest != selected.GenerationDigest || selected.GenerationDigest != digestHex(files["protected-endpoint.json"]) || binding.UID == 0 || binding.GID != gid || len(files) != 15 || len(binding.Files) != 14 {
		return nil, errors.Join(ErrBinding, err)
	}
	names := make([]string, 0, len(files))
	frozen := make(map[string][]byte, len(files))
	allowedNames := map[string]bool{}
	for _, name := range strings.Fields("ardents-linux-amd64 ardents-text-linux-amd64 ardents-text-reader@.service ardents-text-publisher@.service ardents-text-reader.socket ardents-text-publisher.socket 50-ardents-text.rules ardents-text.conf ardents-endpoint.service protected-endpoint.json request.json headless.json source.json endpoint-unit.service binding.json") {
		allowedNames[name] = true
	}
	for name, body := range files {
		if !allowedNames[name] ||
			(name != "binding.json" && binding.Files[name] != digestHex(body)) {
			return nil, ErrBinding
		}
		names = append(names, name)
		frozen[name] = bytes.Clone(body)
	}
	sort.Strings(names)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := lease.observe(); err != nil {
		return nil, err
	}
	stage := &generationStage{lease: lease, selected: selected}
	defer func() {
		if returnedErr != nil {
			stage.retainFailure(ctx, returnedErr)
			returnedErr = stage.close()
		}
	}()
	intentBody, err := canonicalJSON(initialTransitionIntent{Schema: "ardents-endpoint-installation-initial-v1", Candidate: selected, CandidateBinding: binding, Request: *request.declared})
	if err != nil || len(intentBody) > 128<<10 {
		return nil, errors.Join(ErrBinding, err)
	}
	stage.intent, err = writeStagedFile(ctx, lease.root, "transition.json", intentBody, 0600, 0)
	if err != nil {
		return nil, err
	}
	if err := syncStagingRoot(lease.root); err != nil {
		return nil, err
	}
	journalParent, err := stage.birth(ctx, lease.root, "journals")
	if err != nil {
		return nil, err
	}
	stage.journal, err = stage.birth(ctx, journalParent.root, selected.GenerationDigest)
	if err != nil {
		return nil, err
	}
	if err := stage.record(ctx, "writing-generation", nil); err != nil {
		return nil, err
	}
	parent, err := stage.birth(ctx, lease.root, "generations")
	if err != nil {
		return nil, err
	}
	stage.generation, err = stage.birth(ctx, parent.root, selected.GenerationDigest)
	if err != nil {
		return nil, err
	}
	native := stage.generation.identity.Sys().(*syscall.Stat_t)
	birthBody, err := canonicalJSON(generationBirth{Schema: "ardents-endpoint-generation-directory-v1", GenerationDigest: selected.GenerationDigest, Device: uint64(native.Dev), Inode: native.Ino})
	if err != nil {
		return nil, err
	}
	if err := stage.journal.write(ctx, "generation-directory.json", birthBody, 0600, 0); err != nil {
		return nil, err
	}
	for _, name := range names {
		if err := stage.observe(); err != nil {
			return nil, err
		}
		mode := os.FileMode(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		if err := stage.generation.write(ctx, name, frozen[name], mode, gid); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := stage.generation.file.Chown(0, int(gid)); err != nil {
		return nil, err
	}
	if err := stage.generation.file.Chmod(0750); err != nil {
		return nil, err
	}
	if err := stage.generation.file.Sync(); err != nil {
		return nil, err
	}
	promoted, err := stage.generation.file.Stat()
	if err != nil || !os.SameFile(stage.generation.identity, promoted) || promoted.Mode() != os.ModeDir|0750 {
		return nil, errors.Join(ErrBinding, err)
	}
	promotedNative, ok := promoted.Sys().(*syscall.Stat_t)
	if !ok || promotedNative.Uid != 0 || promotedNative.Gid != gid {
		return nil, ErrBinding
	}
	stage.generation.identity = promoted
	if err := syncStagingRoot(parent.root); err != nil {
		return nil, err
	}
	if err := stage.observe(); err != nil {
		return nil, err
	}
	if err := stage.record(ctx, "generation-staged", nil); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return stage, nil
}

func (stage *generationStage) retainFailure(ctx context.Context, original error) {
	if stage.terminal == nil {
		stage.terminal = original
	}
	if stage.journal != nil {
		bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		stage.terminal = errors.Join(stage.terminal, stage.record(bounded, "generation-write-failed", original))
		cancel()
	}
}

func (stage *generationStage) birth(ctx context.Context, parent *os.Root, name string) (*stagingDirectory, error) {
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
	dir := &stagingDirectory{parent: parent, file: file, name: name, files: make(map[string]stagedFile), children: make(map[string]*stagingDirectory)}
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
	for _, ancestor := range stage.directories {
		if ancestor.root == parent {
			ancestor.children[name] = dir
		}
	}
	return dir, errors.Join(file.Sync(), syncStagingRoot(parent))
}

func (stage *generationStage) record(ctx context.Context, phase string, original error) error {
	if (phase != "writing-generation" && phase != "generation-staged" && phase != "generation-write-failed") ||
		((phase == "generation-write-failed") != (original != nil)) {
		return ErrBinding
	}
	name := "0002.json"
	if phase == "writing-generation" {
		name = "0001.json"
	}
	if _, completed := stage.journal.files[name]; completed {
		if original == nil {
			return ErrBinding
		}
		name, phase = "original-transition-failure.json", "successor-transition-failed"
	}
	record := generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: phase}
	if original != nil {
		record.OriginalError = original.Error()
	}
	body, err := canonicalJSON(record)
	if err != nil || len(body) > 64<<10 {
		return errors.Join(ErrBinding, err)
	}
	return stage.journal.write(ctx, name, body, 0600, 0)
}

func (stage *generationStage) observe() (returnedErr error) {
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
	for _, dir := range stage.directories {
		if err := dir.observe(); err != nil {
			return err
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
	return nil
}

func (dir *stagingDirectory) observe() error {
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
	names, readErr := reader.Readdirnames(len(dir.files) + len(dir.children) + 1)
	closeErr := reader.Close()
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	if err := errors.Join(closeErr, readErr); err != nil {
		return err
	}
	if len(names) != len(dir.files)+len(dir.children) {
		return ErrBinding
	}
	for _, name := range names {
		if record, own := dir.files[name]; own {
			if err := observeStagedFile(dir.root, name, record); err != nil {
				return err
			}
		} else if child, own := dir.children[name]; own {
			if err := child.observe(); err != nil {
				return err
			}
		} else {
			return ErrBinding
		}
	}
	for name, record := range dir.files {
		if err := observeStagedFile(dir.root, name, record); err != nil {
			return err
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

func (dir *stagingDirectory) write(ctx context.Context, name string, body []byte, mode os.FileMode, gid uint32) error {
	if err := dir.observe(); err != nil {
		return err
	}
	record, err := writeStagedFile(ctx, dir.root, name, body, mode, gid)
	if record.identity == nil {
		return err
	}
	dir.files[name] = record
	// Finish durability of the already written object even when its caller has
	// just gone. This grants no subsequent write or accepting handoff.
	return errors.Join(err, dir.file.Sync(), ctx.Err())
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

func (stage *generationStage) close() error {
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
	return stage.terminal
}
