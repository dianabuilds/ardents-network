package journal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	posixpath "path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// This native mechanism is private to one preparation operation. The operation
// owns its Installation lease and authenticates inputs before constructing it.
// A retained directory is refused here; recovery has a distinct admission path.
type Owner struct {
	root     *os.Root
	path     string
	identity os.FileInfo
	previous Record
	records  map[string]recordObservation
	failure  error
}

type recordObservation struct {
	identity os.FileInfo
	body     []byte
}

func Create(ctx context.Context, directory string, first Record) (journal *Owner, returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 || !canonicalPath(directory) {
		return nil, ErrUnavailable
	}
	if _, err := Next(Record{}, first); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := rootDirectoryAncestors(filepath.Dir(directory)); err != nil {
		return nil, err
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return nil, err
	}
	// A failure here leaves residue. Never remove it or accept its mere presence
	// as recorded ownership on another attempt.
	if err := os.Chmod(directory, 0700); err != nil {
		return nil, err
	}
	identity, err := os.Lstat(directory)
	if err != nil || !privateJournalDirectory(identity) {
		return nil, errors.Join(ErrUnavailable, err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	j := &Owner{root: root, path: directory, identity: identity, records: make(map[string]recordObservation)}
	defer func() {
		if returnedErr != nil {
			if j.failure == nil {
				j.failure = returnedErr
			}
			returnedErr = j.Close()
		}
	}()
	if err := syncDirectDirectory(filepath.Dir(directory)); err != nil {
		return nil, err
	}
	if err := j.Append(ctx, first); err != nil {
		return nil, err
	}
	return j, nil
}

func (j *Owner) Append(ctx context.Context, record Record) (returnedErr error) {
	if ctx == nil || j == nil {
		return ErrInput
	}
	if j.failure != nil {
		return j.failure
	}
	if j.root == nil {
		return ErrInput
	}
	return j.write(ctx, record)
}

// Failure bookkeeping grants no renewed work. It may retain the original
// operation's error after its caller has gone, but never creates account/roots.
func (j *Owner) RecordFailure(ctx context.Context, original error) error {
	if j == nil || j.root == nil || ctx == nil || original == nil {
		return ErrInput
	}
	record := j.previous
	record.Phase, record.OriginalError = "preparation-failed", original.Error()
	if j.failure == nil {
		j.failure = original
	}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return j.write(bounded, record)
}

func (j *Owner) write(ctx context.Context, record Record) (returnedErr error) {
	defer func() {
		if returnedErr != nil {
			j.failure = errors.Join(j.failure, returnedErr)
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	name, err := Next(j.previous, record)
	if err != nil {
		return err
	}
	body, err := Bytes(record)
	if err != nil {
		return err
	}
	if err := j.observeDirectory(); err != nil {
		return err
	}
	if err := j.observeInventory(); err != nil {
		return err
	}
	file, err := j.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil || !ownedRequestFile(info) || info.Mode().Perm() != 0600 || info.Size() != 0 {
		return errors.Join(ErrUnavailable, err)
	}
	written, err := file.Write(body)
	if err != nil || written != len(body) {
		return errors.Join(io.ErrShortWrite, err)
	}
	if err := file.Sync(); err != nil {
		return err
	}
	writtenInfo, statErr := file.Stat()
	pathInfo, pathErr := j.root.Lstat(name)
	if err := errors.Join(statErr, pathErr); err != nil || !ownedRequestFile(writtenInfo) ||
		writtenInfo.Mode().Perm() != 0600 || writtenInfo.Size() != int64(len(body)) ||
		!os.SameFile(info, writtenInfo) || !sameRequestFile(writtenInfo, pathInfo) {
		return errors.Join(ErrUnavailable, err)
	}
	if err := j.observeDirectory(); err != nil {
		return err
	}
	if err := syncDirectDirectory(j.path); err != nil {
		return err
	}
	// A final caller loss does not erase the durable phase or make it retryable.
	j.previous = record
	j.records[name] = recordObservation{identity: writtenInfo, body: body}
	return ctx.Err()
}

func (j *Owner) observeDirectory() error {
	pathInfo, pathErr := os.Lstat(j.path)
	handleInfo, handleErr := j.root.Stat(".")
	if err := errors.Join(pathErr, handleErr); err != nil || !privateJournalDirectory(pathInfo) ||
		!privateJournalDirectory(handleInfo) || !os.SameFile(j.identity, pathInfo) || !os.SameFile(j.identity, handleInfo) {
		return errors.Join(ErrUnavailable, err)
	}
	for name, record := range j.records {
		if err := j.observeRecord(name, record); err != nil {
			return err
		}
	}
	return nil
}

func (j *Owner) observeInventory() (returnedErr error) {
	directory, err := j.root.Open(".")
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, directory.Close()) }()
	names, err := directory.Readdirnames(5)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(names) != len(j.records) {
		return ErrBinding
	}
	for _, name := range names {
		if _, own := j.records[name]; !own {
			return ErrBinding
		}
	}
	return nil
}

func (j *Owner) observeRecord(name string, record recordObservation) (returnedErr error) {
	info, err := j.root.Lstat(name)
	if err != nil || !sameRequestFile(record.identity, info) {
		return errors.Join(ErrUnavailable, err)
	}
	file, err := j.root.Open(name)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !sameRequestFile(record.identity, opened) {
		return errors.Join(ErrUnavailable, err)
	}
	body, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil || !bytes.Equal(body, record.body) {
		return errors.Join(ErrBinding, err)
	}
	finalHandle, handleErr := file.Stat()
	finalPath, pathErr := j.root.Lstat(name)
	if err := errors.Join(handleErr, pathErr); err != nil || !sameRequestFile(record.identity, finalHandle) || !sameRequestFile(record.identity, finalPath) {
		return errors.Join(ErrUnavailable, err)
	}
	return nil
}

func (j *Owner) Close() error {
	if j == nil {
		return nil
	}
	if j.root == nil {
		return j.failure
	}
	err := j.root.Close()
	j.root = nil
	j.failure = errors.Join(j.failure, err)
	return j.failure
}

func privateJournalDirectory(info os.FileInfo) bool {
	return rootDirectory(info) && info.Mode().Perm() == 0700
}

func syncDirectDirectory(directory string) (returnedErr error) {
	before, err := os.Lstat(directory)
	if err != nil || before == nil || !before.IsDir() || before.Mode().Perm()&0022 != 0 {
		return errors.Join(ErrUnavailable, err)
	}
	owner, ok := before.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != 0 {
		return ErrUnavailable
	}
	file, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return errors.Join(ErrUnavailable, err)
	}
	if err := file.Sync(); err != nil {
		return err
	}
	after, err := os.Lstat(directory)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
		return errors.Join(ErrUnavailable, err)
	}
	return nil
}

// Observe checks the original directory/records and complete finite inventory.
func (j *Owner) Observe() error {
	if j == nil {
		return ErrInput
	}
	if j.root == nil {
		return errors.Join(ErrInput, j.failure)
	}
	return errors.Join(j.observeDirectory(), j.observeInventory())
}
func ownedRequestFile(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0 && native.Nlink == 1
}

func sameRequestFile(before, after os.FileInfo) bool {
	if !ownedRequestFile(before) || !ownedRequestFile(after) || !os.SameFile(before, after) ||
		before.Size() != after.Size() || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		return false
	}
	left, leftOK := before.Sys().(*syscall.Stat_t)
	right, rightOK := after.Sys().(*syscall.Stat_t)
	return leftOK && rightOK && left.Ctim == right.Ctim
}

func rootDirectory(info os.FileInfo) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0
}

func rootDirectoryAncestors(directory string) (map[string]os.FileInfo, error) {
	if !canonicalPath(directory) {
		return nil, ErrUnavailable
	}
	observed := make(map[string]os.FileInfo)
	for {
		info, err := os.Lstat(directory)
		if err != nil || !rootDirectory(info) {
			return nil, errors.Join(ErrUnavailable, err)
		}
		observed[directory] = info
		parent := filepath.Dir(directory)
		if parent == directory {
			return observed, nil
		}
		directory = parent
	}
}

func canonicalPath(value string) bool {
	return posixpath.IsAbs(value) && posixpath.Clean(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}
