package endpoint

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/generation"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
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
	intent           fileObservation
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
		if err != nil || !sameObservedDirectory(expected, info) {
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
		if !sameObservedDirectory(dir.identity, info) {
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

func writeStagedFile(ctx context.Context, root *os.Root, name string, body []byte, mode os.FileMode, gid uint32) (result fileObservation, returnedErr error) {
	if err := ctx.Err(); err != nil {
		return fileObservation{}, err
	}
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return fileObservation{}, err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	birth, err := file.Stat()
	if err != nil || !rootOwnedFile(birth) || birth.Mode().Perm() != 0600 || birth.Size() != 0 {
		return fileObservation{}, errors.Join(ErrBinding, err)
	}
	written, err := file.Write(body)
	if err != nil || written != len(body) {
		return fileObservation{}, errors.Join(io.ErrShortWrite, err)
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return fileObservation{}, err
	}
	if err := file.Chmod(mode); err != nil {
		return fileObservation{}, err
	}
	if err := file.Sync(); err != nil {
		return fileObservation{}, err
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(birth, info) {
		return fileObservation{}, errors.Join(ErrBinding, err)
	}
	result = fileObservation{identity: info, body: bytes.Clone(body), gid: gid, mode: mode}
	return result, errors.Join(observeStagedFile(root, name, result), ctx.Err())
}

func observeStagedFile(root *os.Root, name string, expected fileObservation) (returnedErr error) {
	before, err := root.Lstat(name)
	if err != nil || !observedFileMatches(expected, before) {
		return errors.Join(ErrBinding, err)
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil || !observedFileMatches(expected, info) {
		return errors.Join(ErrBinding, err)
	}
	body, err := io.ReadAll(io.LimitReader(file, int64(len(expected.body))+1))
	if err != nil || !bytes.Equal(body, expected.body) {
		return errors.Join(ErrBinding, err)
	}
	final, err := root.Lstat(name)
	finalHandle, handleErr := file.Stat()
	if err != nil || handleErr != nil || !observedFileMatches(expected, final) || !observedFileMatches(expected, finalHandle) {
		return errors.Join(ErrBinding, err, handleErr)
	}
	return nil
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
		if !sameObservedDirectory(expected, before) || !ok || native.Uid != 0 || native.Gid != gid || before.Mode() != os.ModeDir|0750 {
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
		if !sameObservedDirectory(parent.identity, info) {
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

type generationTransition struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	BindingDigest    string `json:"binding_digest"`
	Phase            string `json:"phase"`
	OriginalError    string `json:"original_error,omitempty"`
}

type generationFileInventory struct {
	Schema           string   `json:"schema"`
	GenerationDigest string   `json:"generation_digest"`
	BindingDigest    string   `json:"binding_digest"`
	Files            []string `json:"files"`
}

type generationFileBirth struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	Name             string `json:"name"`
	Device           uint64 `json:"device"`
	Inode            uint64 `json:"inode"`
	Digest           string `json:"digest"`
	Size             int64  `json:"size"`
	Mode             uint32 `json:"mode"`
	GID              uint32 `json:"gid"`
}

func generationFileRecordName(name string) string {
	return "generation-file-" + digestHex([]byte(name)) + ".json"
}

type generationBirth struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	Device           uint64 `json:"device"`
	Inode            uint64 `json:"inode"`
}

// Only the genuine initial-preparation owner calls this after assembling the
// exact authorized bytes and observing actual account/root provenance.
func stageInitialGeneration(ctx context.Context, lease *installationLease, request Request, files map[string][]byte, selected generationSelection, gid uint32) (result *installationTransaction, returnedErr error) {
	if ctx == nil || lease == nil {
		return nil, ErrBinding
	}
	frozen, binding, err := freezeGenerationInventory(request, lease.path, files, selected, gid)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := lease.observe(); err != nil {
		return nil, err
	}
	stage := &installationTransaction{lease: lease, selected: selected}
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
	journalParent, err := stage.createContainer(ctx, lease.root, "journals")
	if err != nil {
		return nil, err
	}
	stage.journal, err = stage.createTransition(ctx, journalParent.root)
	if err != nil {
		return nil, err
	}
	if err := stage.record(ctx, "writing-generation", nil); err != nil {
		return nil, err
	}
	parent, err := stage.createContainer(ctx, lease.root, "generations")
	if err != nil {
		return nil, err
	}
	stage.generation, err = stage.createGeneration(ctx, parent.root, gid)
	if err != nil {
		return nil, err
	}
	native := stage.generation.Identity()
	birthBody, err := canonicalJSON(generationBirth{Schema: "ardents-endpoint-generation-directory-v1", GenerationDigest: selected.GenerationDigest, Device: native.Device, Inode: native.Inode})
	if err != nil {
		return nil, err
	}
	if err := stage.journal.Write(ctx, journal.Transitions, "generation-directory.json", birthBody); err != nil {
		return nil, err
	}
	if err := stage.writeGeneration(ctx, frozen); err != nil {
		return nil, err
	}
	return stage, nil
}

// Both initial and successor owners reach this only after their exact intent
// and physical directory birth record have become durable.
func (stage *installationTransaction) writeGeneration(ctx context.Context, frozen map[string][]byte) error {
	var binding generationBinding
	if decodeCanonical(frozen["binding.json"], 32<<10, &binding) != nil || binding.GID == 0 {
		return ErrBinding
	}
	marker, err := canonicalJSON(generationFileInventory{Schema: "ardents-endpoint-generation-files-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Files: generation.Names()})
	if err != nil {
		return err
	}
	if err := stage.journal.Write(ctx, journal.Transitions, "generation-files.json", marker); err != nil {
		return err
	}
	names := make([]string, 0, len(frozen))
	for name := range frozen {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := stage.observe(); err != nil {
			return err
		}
		birth, err := stage.generation.CreateFile(ctx, name)
		if err != nil {
			return err
		}
		native, ok := birth.Sys().(*syscall.Stat_t)
		if !ok || native.Uid != 0 || native.Gid != 0 || birth.Mode() != 0600 || birth.Size() != 0 {
			return ErrBinding
		}
		mode := uint32(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		record, err := canonicalJSON(generationFileBirth{Schema: "ardents-endpoint-generation-file-v1", GenerationDigest: stage.selected.GenerationDigest, Name: name, Device: uint64(native.Dev), Inode: native.Ino, Digest: digestHex(frozen[name]), Size: int64(len(frozen[name])), Mode: mode, GID: binding.GID})
		if err != nil {
			return err
		}
		if err := stage.journal.Write(ctx, journal.Transitions, generationFileRecordName(name), record); err != nil {
			return err
		}
		if err := stage.observe(); err != nil {
			return err
		}
		if err := stage.generation.Write(ctx, name, frozen[name]); err != nil {
			return err
		}
	}
	if err := stage.generation.Seal(ctx); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if err := stage.record(ctx, "generation-staged", nil); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (stage *installationTransaction) retainFailure(ctx context.Context, original error) {
	if stage.terminal == nil {
		stage.terminal = original
	}
	if stage.journal != nil {
		bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		stage.terminal = errors.Join(stage.terminal, stage.record(bounded, "generation-write-failed", original))
		cancel()
	}
}

func (stage *installationTransaction) record(ctx context.Context, phase string, original error) error {
	if (phase != "writing-generation" && phase != "generation-staged" && phase != "generation-write-failed") ||
		((phase == "generation-write-failed") != (original != nil)) {
		return ErrBinding
	}
	if original != nil {
		if first := stage.journal.Bytes(journal.Transitions, "original-transition-failure.json"); len(first) != 0 {
			// Preserve the first durable error. Failure-only bookkeeping retains
			// this caller's failure latch without occupying a fresh phase slot.
			return stage.journal.RecordFailure(ctx, "original-transition-failure.json", first, original)
		}
		if failed := stage.journal.Bytes(journal.Transitions, "0002.json"); len(failed) != 0 {
			var first generationTransition
			if decodeCanonical(failed, 64<<10, &first) == nil &&
				first.Schema == "ardents-endpoint-installation-transition-v1" &&
				first.GenerationDigest == stage.selected.GenerationDigest && first.BindingDigest == stage.selected.BindingDigest &&
				first.Phase == "generation-write-failed" && first.OriginalError != "" {
				// A second cleanup must not relabel or replace the first failure.
				return stage.journal.RecordFailure(ctx, "original-transition-failure.json", failed, original)
			}
		}
	}
	name := "0002.json"
	if phase == "writing-generation" {
		name = "0001.json"
	}
	if len(stage.journal.Bytes(journal.Transitions, name)) != 0 {
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
	if original != nil {
		return stage.journal.RecordFailure(ctx, name, body, original)
	}
	return stage.journal.Write(ctx, journal.Transitions, name, body)
}

func (stage *installationTransaction) promoteAccess(ctx context.Context, gid uint32) error {
	if ctx == nil || gid == 0 || gid == 1<<32-1 {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	native := stage.generation.Identity()
	if native.GID != gid {
		return ErrBinding
	}
	if len(stage.journal.Bytes(journal.Transitions, "0004.json")) == 0 {
		return ErrBinding
	}
	var generations *installationContainer
	for _, directory := range stage.directories {
		if directory.parent == stage.lease.root && directory.name == "generations" {
			generations = directory
		}
	}
	if generations == nil {
		return ErrBinding
	}
	info, err := promoteReadDirectory(ctx, filepath.Join(stage.lease.path, "generations"), generations.root, generations.identity, gid)
	if info != nil {
		generations.identity = info
	}
	if err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	info, err = promoteReadDirectory(ctx, stage.lease.path, stage.lease.root, stage.lease.identity, gid)
	if info != nil {
		stage.lease.identity = info
	}
	if err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	return ctx.Err()
}

func promoteReadDirectory(ctx context.Context, path string, root *os.Root, original os.FileInfo, gid uint32) (result os.FileInfo, returnedErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pathInfo, pathErr := os.Lstat(path)
	rootInfo, rootErr := root.Stat(".")
	if pathErr != nil || rootErr != nil || !sameObservedDirectory(original, pathInfo) || !sameObservedDirectory(original, rootInfo) {
		return nil, errors.Join(ErrBinding, pathErr, rootErr)
	}
	native, ok := original.Sys().(*syscall.Stat_t)
	if !ok || gid == 0 || gid == 1<<32-1 {
		return nil, ErrBinding
	}
	if original.Mode() == os.ModeDir|0750 && native.Gid == gid {
		return original, ctx.Err()
	}
	if original.Mode() != os.ModeDir|0700 || native.Gid != 0 {
		return nil, ErrBinding
	}
	file, err := root.OpenFile(".", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	before, err := file.Stat()
	if err != nil || !sameObservedDirectory(original, before) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return nil, err
	}
	if err := file.Chmod(0750); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(original, info) || info.Mode() != os.ModeDir|0750 {
		return nil, errors.Join(ErrBinding, err)
	}
	actual, ok := info.Sys().(*syscall.Stat_t)
	if !ok || actual.Uid != 0 || actual.Gid != gid {
		return nil, ErrBinding
	}
	pathInfo, pathErr = os.Lstat(path)
	rootInfo, rootErr = root.Stat(".")
	if pathErr != nil || rootErr != nil || !sameObservedDirectory(info, pathInfo) || !sameObservedDirectory(info, rootInfo) {
		return nil, errors.Join(ErrBinding, pathErr, rootErr)
	}
	// Preserve the changed same inode even if the original caller goes after
	// durability. Its owner returns refusal and retains explicit failure intent.
	return info, ctx.Err()
}

// Creation and recovery retain different physical owners. Recovery reads a
// sealed Snapshot; it never fabricates a generation's exclusive birth Owner.
func (stage *installationTransaction) generationBytes(name string) []byte {
	if stage == nil {
		return nil
	}
	if stage.generation != nil {
		return stage.generation.Bytes(name)
	}
	return stage.sealed.Bytes(name)
}

func (stage *installationTransaction) generationIdentity() generation.Identity {
	if stage == nil {
		return generation.Identity{}
	}
	if stage.generation != nil {
		return stage.generation.Identity()
	}
	if stage.prefix != nil {
		return stage.prefix.Identity()
	}
	if stage.sealed == nil || stage.sealedIdentity == nil {
		return generation.Identity{}
	}
	info, ok := stage.sealedIdentity.Sys().(*syscall.Stat_t)
	if !ok {
		return generation.Identity{}
	}
	return generation.Identity{Device: uint64(info.Dev), Inode: info.Ino, GID: info.Gid, Mode: stage.sealedIdentity.Mode()}
}

// A complete sealed prefix may have the old directory-only provenance, or all
// fifteen new exact file births. A partial new set cannot look like an old one.
// These facts constrain original custody; fresh Release proofs remain separate.
func readGenerationFileBirths(ctx context.Context, reader *installedRoot, directory string, selected generationSelection, binding generationBinding, expected map[string][]byte, inventory map[string]bool) (map[string]journal.RetainedRecord, error) {
	records := make(map[string]journal.RetainedRecord)
	markerPath := filepath.Join(directory, "generation-files.json")
	marker, markerErr := reader.read(ctx, markerPath, 4<<10, 0600, 0)
	present := !errors.Is(markerErr, os.ErrNotExist)
	if present {
		expectedMarker, err := canonicalJSON(generationFileInventory{Schema: "ardents-endpoint-generation-files-v1", GenerationDigest: selected.GenerationDigest, BindingDigest: selected.BindingDigest, Files: generation.Names()})
		if markerErr != nil || err != nil || !bytes.Equal(marker, expectedMarker) {
			return nil, errors.Join(ErrBinding, markerErr, err)
		}
		inventory["generation-files.json"] = true
		records["generation-files.json"] = journal.RetainedRecord{Identity: reader.files[markerPath].identity, Bytes: marker}
	}
	for _, name := range generation.Names() {
		path := filepath.Join(directory, generationFileRecordName(name))
		body, err := reader.read(ctx, path, 4<<10, 0600, 0)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if !present {
			return nil, ErrBinding
		}
		var birth generationFileBirth
		mode := uint32(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		digest := binding.Files[name]
		if name == "binding.json" {
			digest = selected.BindingDigest
		}
		if err != nil || decodeCanonical(body, 4<<10, &birth) != nil || birth.Schema != "ardents-endpoint-generation-file-v1" || birth.GenerationDigest != selected.GenerationDigest || birth.Name != name || birth.Digest != digest || !canonicalDigest(digest) || birth.Size < 1 || birth.Size > 64<<20 || birth.Mode != mode || birth.GID != reader.gid || birth.Device == 0 || birth.Inode == 0 {
			return nil, errors.Join(ErrBinding, err)
		}
		if name == "binding.json" {
			encoded, err := canonicalJSON(binding)
			if err != nil || birth.Size != int64(len(encoded)) {
				return nil, errors.Join(ErrBinding, err)
			}
		} else if expected != nil && (len(expected[name]) == 0 || birth.Size != int64(len(expected[name]))) {
			return nil, ErrBinding
		}
		filename := filepath.Join(reader.lease.path, "generations", selected.GenerationDigest, name)
		actual, err := reader.readObserved(ctx, filename, birth.Size, os.FileMode(mode), reader.gid, true)
		observed := reader.files[filename]
		if err != nil || observed.identity == nil || int64(len(actual)) > birth.Size {
			return nil, errors.Join(ErrBinding, err)
		}
		native, ok := observed.identity.Sys().(*syscall.Stat_t)
		if !ok || uint64(native.Dev) != birth.Device || native.Ino != birth.Inode || int64(len(actual)) == birth.Size && digestHex(actual) != birth.Digest {
			return nil, ErrBinding
		}
		recordName := generationFileRecordName(name)
		inventory[recordName] = true
		records[recordName] = journal.RetainedRecord{Identity: reader.files[path].identity, Bytes: body}
	}
	if present && len(records) != len(generation.Names())+1 {
		return nil, ErrBinding
	}
	return records, ctx.Err()
}

// Copy/sync the exact owned intent before removing its original inode. A
// retained archive is refused here; explicit recovery has separate admission.
func (stage *installationTransaction) archiveIntent(ctx context.Context) error {
	if ctx == nil || stage == nil || stage.journal == nil {
		return ErrInput
	}
	// Initial stopped completion cannot authorize successor archival. That
	// operation must observe its actual started invocation and completion peer.
	var intent initialTransitionIntent
	if decodeCanonical(stage.intent.body, 128<<10, &intent) != nil ||
		intent.Schema != "ardents-endpoint-installation-initial-v1" || intent.Candidate != stage.selected {
		return ErrBinding
	}
	if err := stage.verifyTransitionPhase("0007.json", "installed-stopped"); err != nil {
		return err
	}
	return stage.copyAndRemoveIntent(ctx)
}

// Physical archival only. Each caller admits its own exact completion before
// reaching this mechanism; an archive is not a process or startup proof.
func (stage *installationTransaction) copyAndRemoveIntent(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if len(stage.journal.Bytes(journal.Transitions, "0007.json")) == 0 || stage.archivedIntent {
		return ErrBinding
	}
	if err := stage.journal.Write(ctx, journal.Transitions, "completed-intent.json", stage.intent.body); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := observeStagedFile(stage.lease.root, "transition.json", stage.intent); err != nil {
		return err
	}
	if err := stage.lease.root.Remove("transition.json"); err != nil {
		return err
	}
	stage.archivedIntent = true
	if err := syncStagingRoot(stage.lease.root); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	return ctx.Err()
}

func (stage *installationTransaction) observeIntent() error {
	if !stage.archivedIntent {
		return observeStagedFile(stage.lease.root, "transition.json", stage.intent)
	}
	if _, err := stage.lease.root.Lstat("transition.json"); !os.IsNotExist(err) {
		return errors.Join(ErrBinding, err)
	}
	if !bytes.Equal(stage.journal.Bytes(journal.Transitions, "completed-intent.json"), stage.intent.body) {
		return ErrBinding
	}
	if err := stage.journal.Observe(); err != nil {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

// A native byte-staging mechanism, not fresh proof admission. Its only product
// caller is the still-leased preparation with its genuine retained fresh pair.
func stageSuccessorGeneration(ctx context.Context, reader *installedRoot, request Request, previous generationSelection, files map[string][]byte, selected generationSelection, gid uint32) (result *installationTransaction, returnedErr error) {
	if ctx == nil || reader == nil || reader.lease == nil || request.declared == nil || request.ManifestSHA256() != "" ||
		previous.Schema != "ardents-endpoint-installation-selection-v1" || !canonicalDigest(previous.GenerationDigest) ||
		!canonicalDigest(previous.BindingDigest) || previous.GenerationDigest == selected.GenerationDigest {
		return nil, ErrBinding
	}
	lease := reader.lease
	frozen, binding, err := freezeGenerationInventory(request, lease.path, files, selected, gid)
	if err != nil {
		return nil, err
	}
	previousRaw, err := canonicalJSON(previous)
	selectionPath := filepath.Join(lease.path, "selection.json")
	selection := reader.files[selectionPath]
	if err != nil || selection.identity == nil || selection.gid != gid || selection.mode != 0640 || !bytes.Equal(selection.body, previousRaw) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := reader.observe(ctx); err != nil {
		return nil, err
	}
	stage := &installationTransaction{lease: lease, selected: selected, fixed: map[string]fixedFileObservation{selectionPath: {parent: lease.identity, file: selection}}}
	defer func() {
		if returnedErr != nil {
			stage.retainFailure(ctx, returnedErr)
			if stage.intentAttempted {
				result = stage
				returnedErr = stage.terminal
			} else {
				returnedErr = stage.close()
			}
		}
	}()
	journalParent, err := stage.borrowParent(ctx, "journals", gid, nil)
	if err != nil {
		return nil, err
	}
	generationParent, err := stage.borrowParent(ctx, "generations", gid, reader.directories[filepath.Join(lease.path, "generations")])
	if err != nil {
		return nil, err
	}
	// Refuse both forms of candidate residue before writing even the intent.
	for _, parent := range []*stagingParent{journalParent, generationParent} {
		if _, err := parent.root.Lstat(selected.GenerationDigest); !errors.Is(err, syscall.ENOENT) {
			return nil, errors.Join(ErrBinding, err)
		}
	}
	intent := successorTransitionIntent{Schema: "ardents-endpoint-installation-successor-v1", Previous: previous, Candidate: selected, CandidateBinding: binding, Request: *request.declared}
	intentBody, err := canonicalJSON(intent)
	if err != nil || len(intentBody) > 128<<10 {
		return nil, errors.Join(ErrBinding, err)
	}
	// Even a failed exclusive write may have created an incomplete original
	// inode. Quiescence custody starts before the attempt, not at successful sync.
	stage.intentAttempted = true
	stage.intent, err = writeStagedFile(ctx, lease.root, "transition.json", intentBody, 0600, 0)
	if err != nil {
		return nil, err
	}
	if err := syncStagingRoot(lease.root); err != nil {
		return nil, err
	}
	stage.journal, err = stage.createTransition(ctx, journalParent.root)
	if err != nil {
		return nil, err
	}
	if err := stage.record(ctx, "writing-generation", nil); err != nil {
		return nil, err
	}
	stage.generation, err = stage.createGeneration(ctx, generationParent.root, gid)
	if err != nil {
		return nil, err
	}
	native := stage.generation.Identity()
	birth, err := canonicalJSON(generationBirth{Schema: "ardents-endpoint-generation-directory-v1", GenerationDigest: selected.GenerationDigest, Device: native.Device, Inode: native.Inode})
	if err != nil {
		return nil, err
	}
	if err := stage.journal.Write(ctx, journal.Transitions, "generation-directory.json", birth); err != nil {
		return nil, err
	}
	if err := stage.writeGeneration(ctx, frozen); err != nil {
		return nil, err
	}
	if err := reader.observe(ctx); err != nil {
		return nil, err
	}
	return stage, nil
}

func (stage *installationTransaction) verifyTransitionPhase(name, phase string) error {
	if stage == nil || stage.journal == nil {
		return ErrInput
	}
	var observed generationTransition
	if err := decodeCanonical(stage.journal.Bytes(journal.Transitions, name), 64<<10, &observed); err != nil || observed.Schema != "ardents-endpoint-installation-transition-v1" ||
		observed.GenerationDigest != stage.selected.GenerationDigest || observed.BindingDigest != stage.selected.BindingDigest || observed.Phase != phase || observed.OriginalError != "" {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

// Physical intention only. Its transaction caller retains fresh proofs, exact
// complete fixed images and actual predecessor quiescence before selection.
func (stage *installationTransaction) recordSelectionPublication(ctx context.Context) error {
	if ctx == nil || stage == nil || stage.journal == nil {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.verifyTransitionPhase("0004.json", "fixed-resources-replaced"); err != nil {
		return err
	}
	if len(stage.journal.Bytes(journal.Transitions, "0005.json")) != 0 {
		if err := stage.verifyTransitionPhase("0005.json", "publishing-selection"); err != nil {
			return err
		}
		return stage.journal.Resync(ctx, journal.Transitions, "0005.json")
	}
	phase, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: "publishing-selection"})
	if err != nil {
		return err
	}
	return stage.journal.Write(ctx, journal.Transitions, "0005.json", phase)
}
