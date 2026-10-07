package installation

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// This native mechanism is private to one preparation operation. The operation
// owns its Installation lease and authenticates inputs before constructing it.
// A retained directory is refused here; recovery has a distinct admission path.
type preparationJournal struct {
	root     *os.Root
	path     string
	identity os.FileInfo
	previous preparationRecord
	records  map[string]preparationObservation
	failure  error
}

type preparationObservation struct {
	identity os.FileInfo
	body     []byte
}

func createPreparationJournal(ctx context.Context, directory string, first preparationRecord) (journal *preparationJournal, returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 || !canonicalPath(directory) {
		return nil, ErrNativeUnavailable
	}
	if _, err := preparationNext(preparationRecord{}, first); err != nil {
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
		return nil, errors.Join(ErrNativeUnavailable, err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	j := &preparationJournal{root: root, path: directory, identity: identity, records: make(map[string]preparationObservation)}
	defer func() {
		if returnedErr != nil {
			if j.failure == nil {
				j.failure = returnedErr
			}
			returnedErr = j.close()
		}
	}()
	if err := syncDirectDirectory(filepath.Dir(directory)); err != nil {
		return nil, err
	}
	if err := j.append(ctx, first); err != nil {
		return nil, err
	}
	return j, nil
}

func (j *preparationJournal) append(ctx context.Context, record preparationRecord) (returnedErr error) {
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
func (j *preparationJournal) recordFailure(ctx context.Context, original error) error {
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

func (j *preparationJournal) write(ctx context.Context, record preparationRecord) (returnedErr error) {
	defer func() {
		if returnedErr != nil {
			j.failure = errors.Join(j.failure, returnedErr)
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	name, err := preparationNext(j.previous, record)
	if err != nil {
		return err
	}
	body, err := preparationRecordBytes(record)
	if err != nil {
		return err
	}
	if err := j.observe(); err != nil {
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
		return errors.Join(ErrNativeUnavailable, err)
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
		return errors.Join(ErrNativeUnavailable, err)
	}
	if err := j.observe(); err != nil {
		return err
	}
	if err := syncDirectDirectory(j.path); err != nil {
		return err
	}
	// A final caller loss does not erase the durable phase or make it retryable.
	j.previous = record
	j.records[name] = preparationObservation{identity: writtenInfo, body: body}
	return ctx.Err()
}

func (j *preparationJournal) observe() error {
	pathInfo, pathErr := os.Lstat(j.path)
	handleInfo, handleErr := j.root.Stat(".")
	if err := errors.Join(pathErr, handleErr); err != nil || !privateJournalDirectory(pathInfo) ||
		!privateJournalDirectory(handleInfo) || !os.SameFile(j.identity, pathInfo) || !os.SameFile(j.identity, handleInfo) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	for name, record := range j.records {
		if err := j.observeRecord(name, record); err != nil {
			return err
		}
	}
	return nil
}

func (j *preparationJournal) observeInventory() (returnedErr error) {
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

func (j *preparationJournal) observeRecord(name string, record preparationObservation) (returnedErr error) {
	info, err := j.root.Lstat(name)
	if err != nil || !sameRequestFile(record.identity, info) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	file, err := j.root.Open(name)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !sameRequestFile(record.identity, opened) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	body, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil || !bytes.Equal(body, record.body) {
		return errors.Join(ErrBinding, err)
	}
	finalHandle, handleErr := file.Stat()
	finalPath, pathErr := j.root.Lstat(name)
	if err := errors.Join(handleErr, pathErr); err != nil || !sameRequestFile(record.identity, finalHandle) || !sameRequestFile(record.identity, finalPath) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	return nil
}

func (j *preparationJournal) close() error {
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
		return errors.Join(ErrNativeUnavailable, err)
	}
	owner, ok := before.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != 0 {
		return ErrNativeUnavailable
	}
	file, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	if err := file.Sync(); err != nil {
		return err
	}
	after, err := os.Lstat(directory)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
		return errors.Join(ErrNativeUnavailable, err)
	}
	return nil
}
