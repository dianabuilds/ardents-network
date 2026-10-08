package journal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Collection names one closed group of original transition provenance. It is
// neither a pathname supplied by a caller nor an Installation phase decision.
type Collection uint8

const (
	Transitions Collection = iota
	Creations
	DirectoryCreations
	Replacements
)

// Transition owns one exclusively born journal and its original record groups.
// Installation serializes calls and retains admission, caller, lease and phase
// policy. No method adopts residue, changes fixed resources or admits startup.
// A post-birth error returns partial custody until the operation physically joins.
type Transition struct {
	parent         *os.Root
	parentPath     string
	parentIdentity os.FileInfo
	name           string
	directory      *transitionDirectory
	groups         map[Collection]*transitionDirectory
	order          []*transitionDirectory
	failure        error
}
type transitionDirectory struct {
	parent, root *os.Root
	file         *os.File
	name         string
	identity     os.FileInfo
	records      map[string]recordObservation
	children     map[string]*transitionDirectory
}

func CreateTransition(ctx context.Context, parentPath string, expected os.FileInfo, name string) (result *Transition, returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 || !canonicalPath(parentPath) || filepath.Base(parentPath) != "journals" || !canonicalDigest(name) || !privateJournalDirectory(expected) {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, err
	}
	j := &Transition{parent: parent, parentPath: parentPath, parentIdentity: expected, name: name, groups: make(map[Collection]*transitionDirectory)}
	born := false
	defer func() {
		if returnedErr != nil {
			j.failure = returnedErr
			if born {
				result = j
			} else {
				returnedErr = errors.Join(returnedErr, parent.Close())
				j.parent = nil
			}
		}
	}()
	if err := j.observeParent(); err != nil {
		return nil, err
	}
	if err := parent.Mkdir(name, 0700); err != nil {
		return nil, err
	}
	born = true
	j.directory = &transitionDirectory{parent: parent, name: name, records: make(map[string]recordObservation), children: make(map[string]*transitionDirectory)}
	if err := j.directory.open(); err != nil {
		return nil, err
	}
	if err := errors.Join(j.directory.file.Sync(), transitionSync(parent), j.observePhysical(), ctx.Err()); err != nil {
		return nil, err
	}
	return j, nil
}

// Ensure establishes an original record-group directory before the associated
// fixed-object birth. It never exports a root or accepts a foreign existing group.
func (j *Transition) Ensure(ctx context.Context, group Collection) (returnedErr error) {
	if ctx == nil || j == nil {
		return ErrInput
	}
	if j.failure != nil {
		return j.failure
	}
	defer func() {
		if returnedErr != nil {
			j.failure = returnedErr
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	name := collectionName(group)
	if name == "" || group == Transitions {
		return ErrInput
	}
	if err := j.observePhysical(); err != nil {
		return err
	}
	if j.groups[group] != nil {
		return ctx.Err()
	}
	if err := j.directory.root.Mkdir(name, 0700); err != nil {
		return err
	}
	dir := &transitionDirectory{parent: j.directory.root, name: name, records: make(map[string]recordObservation), children: make(map[string]*transitionDirectory)}
	// Register partial custody before any opening can fail. No subsequent call
	// may forget the born directory or renew a failed journal with a fresh caller.
	j.groups[group] = dir
	j.order = append(j.order, dir)
	j.directory.children[name] = dir
	if err := dir.open(); err != nil {
		return err
	}
	return errors.Join(dir.file.Sync(), j.directory.file.Sync(), j.observePhysical(), ctx.Err())
}

func (j *Transition) Write(ctx context.Context, group Collection, name string, body []byte) error {
	if j == nil || ctx == nil {
		return ErrInput
	}
	if j.failure != nil {
		return j.failure
	}
	err := j.write(ctx, group, name, body)
	if err != nil {
		j.failure = err
	}
	return err
}

// RecordFailure is failure-only bookkeeping under the same original custody.
// It cannot renew ordinary Write, establish a group or erase the first failure.
func (j *Transition) RecordFailure(ctx context.Context, name string, body []byte, original error) error {
	if j == nil || ctx == nil || original == nil || (name != "0002.json" && name != "original-transition-failure.json") {
		return ErrInput
	}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if j.failure == nil {
		j.failure = original
	}
	err := j.write(bounded, Transitions, name, body)
	j.failure = errors.Join(j.failure, err)
	return err
}

func (j *Transition) write(ctx context.Context, group Collection, name string, body []byte) (returnedErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !transitionName(group, name) || len(body) == 0 || len(body) > 128<<10 || name != "completed-intent.json" && len(body) > 64<<10 {
		return ErrInput
	}
	if err := j.observePhysical(); err != nil {
		return err
	}
	dir := j.group(group)
	if dir == nil {
		return ErrInput
	}
	if _, exists := dir.records[name]; exists {
		return ErrBinding
	}
	if group == Transitions && generationFileRecord(name) {
		births := 0
		for existing := range dir.records {
			if generationFileRecord(existing) {
				births++
			}
		}
		if births >= 15 || len(body) > 4<<10 {
			return ErrInput
		}
	}
	file, err := dir.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	birth, err := file.Stat()
	if err != nil || !ownedRequestFile(birth) || birth.Mode().Perm() != 0600 || birth.Size() != 0 {
		return errors.Join(ErrBinding, err)
	}
	frozen := bytes.Clone(body)
	written, err := file.Write(frozen)
	if err != nil || written != len(frozen) {
		return errors.Join(io.ErrShortWrite, err)
	}
	if err := file.Chown(0, 0); err != nil {
		return err
	}
	if err := file.Chmod(0600); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	current, err := file.Stat()
	if err != nil || !os.SameFile(birth, current) {
		return errors.Join(ErrBinding, err)
	}
	record := recordObservation{identity: current, body: frozen}
	// Retain the visible original before final caller/durability refusal, exactly
	// as the transaction did before extraction. Visibility does not attest sync.
	dir.records[name] = record
	observedErr := dir.observeRecord(name, record)
	return errors.Join(observedErr, dir.file.Sync(), ctx.Err())
}

// Bytes and HasCollection expose detached bookkeeping facts, including after a
// failure. Callers still need original Observe/admission before any effect.
func (j *Transition) Bytes(group Collection, name string) []byte {
	if j == nil || j.parent == nil {
		return nil
	}
	dir := j.group(group)
	if dir == nil {
		return nil
	}
	return bytes.Clone(dir.records[name].body)
}
func (j *Transition) HasCollection(group Collection) bool {
	return j != nil && j.parent != nil && j.group(group) != nil
}
func (j *Transition) group(group Collection) *transitionDirectory {
	if group == Transitions {
		return j.directory
	}
	return j.groups[group]
}

// Resync observes and synchronizes the same recorded file and directory before
// each authorized fixed-resource mutation; prior visibility is never sufficient.
func (j *Transition) Resync(ctx context.Context, group Collection, name string) (returnedErr error) {
	if ctx == nil || j == nil {
		return ErrInput
	}
	if j.failure != nil {
		return j.failure
	}
	defer func() {
		if returnedErr != nil {
			j.failure = returnedErr
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := j.observePhysical(); err != nil {
		return err
	}
	dir := j.group(group)
	if dir == nil {
		return ErrInput
	}
	record, found := dir.records[name]
	if !found {
		return ErrInput
	}
	file, err := dir.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close(), ctx.Err()) }()
	info, err := file.Stat()
	if err != nil || !transitionRecordMatches(record, info) {
		return errors.Join(ErrBinding, err)
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := dir.file.Sync(); err != nil {
		return err
	}
	return errors.Join(j.observePhysical(), ctx.Err())
}

func (j *Transition) Observe() (returnedErr error) {
	if j == nil {
		return ErrInput
	}
	if j.failure != nil {
		return j.failure
	}
	defer func() {
		if returnedErr != nil {
			j.failure = returnedErr
		}
	}()
	return j.observePhysical()
}
func (j *Transition) observeParent() error {
	if j.parent == nil {
		return ErrBinding
	}
	path, pathErr := os.Lstat(j.parentPath)
	root, rootErr := j.parent.Stat(".")
	if pathErr != nil || rootErr != nil || !transitionDirectoryMatches(j.parentIdentity, path) || !transitionDirectoryMatches(j.parentIdentity, root) {
		return errors.Join(ErrBinding, pathErr, rootErr)
	}
	return nil
}
func (j *Transition) observePhysical() error {
	if j == nil || j.directory == nil {
		return ErrBinding
	}
	if err := j.observeParent(); err != nil {
		return err
	}
	if err := j.directory.observe(); err != nil {
		return err
	}
	return j.observeParent()
}
func (dir *transitionDirectory) open() error {
	if err := dir.openHandles(); err != nil {
		return err
	}
	return dir.observe()
}
func (dir *transitionDirectory) openHandles() error {
	file, err := dir.parent.OpenFile(dir.name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	dir.file = file
	dir.identity, err = file.Stat()
	if err != nil || !privateJournalDirectory(dir.identity) {
		return errors.Join(ErrBinding, err)
	}
	dir.root, err = dir.parent.OpenRoot(dir.name)
	if err != nil {
		return err
	}
	return nil
}
func (dir *transitionDirectory) observe() error {
	if dir == nil || dir.parent == nil || dir.root == nil || dir.file == nil || dir.identity == nil {
		return ErrBinding
	}
	path, pathErr := dir.parent.Lstat(dir.name)
	root, rootErr := dir.root.Stat(".")
	fd, fdErr := dir.file.Stat()
	for _, info := range []os.FileInfo{path, root, fd} {
		if !transitionDirectoryMatches(dir.identity, info) {
			return errors.Join(ErrBinding, pathErr, rootErr, fdErr)
		}
	}
	reader, err := dir.root.Open(".")
	if err != nil {
		return err
	}
	names, readErr := reader.Readdirnames(len(dir.records) + len(dir.children) + 1)
	closeErr := reader.Close()
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	if len(names) != len(dir.records)+len(dir.children) {
		return ErrBinding
	}
	for _, name := range names {
		if record, own := dir.records[name]; own {
			if err := dir.observeRecord(name, record); err != nil {
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
	for name, record := range dir.records {
		if err := dir.observeRecord(name, record); err != nil {
			return err
		}
	}
	return nil
}
func (dir *transitionDirectory) observeRecord(name string, record recordObservation) (returnedErr error) {
	before, err := dir.root.Lstat(name)
	if err != nil || !transitionRecordMatches(record, before) {
		return errors.Join(ErrBinding, err)
	}
	file, err := dir.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !transitionRecordMatches(record, opened) {
		return errors.Join(ErrBinding, err)
	}
	body, err := io.ReadAll(io.LimitReader(file, int64(len(record.body))+1))
	if err != nil || !bytes.Equal(body, record.body) {
		return errors.Join(ErrBinding, err)
	}
	final, pathErr := dir.root.Lstat(name)
	fd, fdErr := file.Stat()
	if pathErr != nil || fdErr != nil || !transitionRecordMatches(record, final) || !transitionRecordMatches(record, fd) {
		return errors.Join(ErrBinding, pathErr, fdErr)
	}
	return nil
}
func transitionRecordMatches(record recordObservation, info os.FileInfo) bool {
	if record.identity == nil || !ownedRequestFile(info) || !os.SameFile(record.identity, info) || info.Mode() != 0600 || info.Size() != int64(len(record.body)) || !info.ModTime().Equal(record.identity.ModTime()) {
		return false
	}
	original, a := record.identity.Sys().(*syscall.Stat_t)
	current, b := info.Sys().(*syscall.Stat_t)
	return a && b && original.Ctim == current.Ctim
}
func transitionDirectoryMatches(original, current os.FileInfo) bool {
	return original != nil && privateJournalDirectory(original) && privateJournalDirectory(current) && os.SameFile(original, current) && original.Mode() == current.Mode()
}
func transitionSync(root *os.Root) (returnedErr error) {
	file, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	return file.Sync()
}
func collectionName(group Collection) string {
	switch group {
	case Transitions:
		return ""
	case Creations:
		return "creations"
	case DirectoryCreations:
		return "directory-creations"
	case Replacements:
		return "replacements"
	}
	return ""
}
func transitionName(group Collection, name string) bool {
	if group == Transitions {
		if generationFileRecord(name) {
			return true
		}
		switch name {
		case "0001.json", "0002.json", "0003.json", "0004.json", "0005.json", "0006.json", "0007.json", "generation-directory.json", "generation-files.json", "start-attempt.json", "started-invocation.json", "completed-intent.json", "original-transition-failure.json", "completion-socket-removal.json", "completion-record-removal.json", "completion-guard-removal.json":
			return true
		}
		return false
	}
	if collectionName(group) == "" || !strings.HasSuffix(name, ".json") {
		return false
	}
	stem := strings.TrimSuffix(name, ".json")
	if group == DirectoryCreations {
		stem = strings.TrimSuffix(stem, "-access")
	}
	return canonicalDigest(stem)
}

// File-birth provenance has its own finite namespace. Installation alone
// validates the artifact roster, record schema and admitted mutation ordering.
func generationFileRecord(name string) bool {
	return strings.HasPrefix(name, "generation-file-") && strings.HasSuffix(name, ".json") && canonicalDigest(strings.TrimSuffix(strings.TrimPrefix(name, "generation-file-"), ".json"))
}

func (j *Transition) Close() error {
	if j == nil {
		return nil
	}
	for index := len(j.order) - 1; index >= 0; index-- {
		j.failure = errors.Join(j.failure, j.order[index].close())
	}
	if j.directory != nil {
		j.failure = errors.Join(j.failure, j.directory.close())
	}
	if j.parent != nil {
		j.failure = errors.Join(j.failure, j.parent.Close())
		j.parent = nil
	}
	return j.failure
}
func (dir *transitionDirectory) close() error {
	if dir == nil {
		return nil
	}
	var result error
	if dir.root != nil {
		result = errors.Join(result, dir.root.Close())
		dir.root = nil
	}
	if dir.file != nil {
		result = errors.Join(result, dir.file.Close())
		dir.file = nil
	}
	return result
}
