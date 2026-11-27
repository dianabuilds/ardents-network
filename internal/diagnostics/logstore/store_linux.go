package logstore

import (
	"errors"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
)

// Policy bounds the private log directory. MaxFiles includes
// its empty lock file. Counters are session-local; retained bytes survive restart.
type Policy struct {
	SegmentBytes int64         `json:"segment_bytes"`
	MaxBytes     int64         `json:"max_bytes"`
	MaxFiles     int           `json:"max_files"`
	SegmentAge   time.Duration `json:"segment_age_ns"`
	MaxAge       time.Duration `json:"max_age_ns"`
}

// Stats describes the original store's retained inventory and session accounting.
type Stats struct {
	FilesystemAvailable  bool  `json:"filesystem_available"`
	FilesystemTotalBytes int64 `json:"filesystem_total_bytes"`
	FilesystemFreeBytes  int64 `json:"filesystem_available_bytes"`
	ReceivedBytes        int64 `json:"received_bytes"`
	WrittenBytes         int64 `json:"written_bytes"`
	LostBytes            int64 `json:"lost_bytes"`
	ExpiredBytes         int64 `json:"expired_bytes"`
	RetainedBytes        int64 `json:"retained_bytes"`
	Files                int   `json:"files"`
	Failed               bool  `json:"failed"`
}
type retainedLogSegment struct {
	name, stream string
	sequence     uint64
	born         time.Time
	size         int64
	info         os.FileInfo
	file         *os.File
}

// Store retains the original Linux directory and segment writer lease.
type Store struct {
	mu       sync.Mutex
	root     *os.Root
	lock     *os.File
	policy   Policy
	segments []*retainedLogSegment
	active   map[string]*retainedLogSegment
	next     uint64
	stats    Stats
	failure  error
	closed   bool
}

// Policy returns the immutable limits of this store.
func (store *Store) Policy() Policy { return store.policy }

// Continue reopens the closed store for maintenance in the same monitor
// session. Inventory is reobserved while delivery counters stay cumulative.
func (store *Store) Continue(at time.Time) (*Store, error) {
	store.mu.Lock()
	if !store.closed || store.root == nil {
		store.mu.Unlock()
		return nil, errors.New("log store must close before maintenance")
	}
	path, policy, previous := store.root.Name(), store.policy, store.stats
	store.mu.Unlock()
	next, err := Open(path, policy, at)
	if err != nil {
		return nil, err
	}
	next.stats.ReceivedBytes = previous.ReceivedBytes
	next.stats.WrittenBytes = previous.WrittenBytes
	next.stats.LostBytes = previous.LostBytes
	next.stats.ExpiredBytes += previous.ExpiredBytes
	return next, nil
}

func (store *Store) remove(segment *retainedLogSegment) error {
	info, err := store.root.Lstat(segment.name)
	if err != nil || !IsPrivateFile(info) || !os.SameFile(segment.info, info) {
		return errors.Join(err, errors.New("log segment ownership changed"))
	}
	if segment.file != nil {
		if err := segment.file.Close(); err != nil {
			return err
		}
		segment.file = nil
	}
	if err := store.root.Remove(segment.name); err != nil {
		return err
	}
	if store.active[segment.stream] == segment {
		delete(store.active, segment.stream)
	}
	for i, current := range store.segments {
		if current == segment {
			store.segments = append(store.segments[:i], store.segments[i+1:]...)
			break
		}
	}
	store.stats.RetainedBytes -= segment.size
	store.stats.ExpiredBytes += segment.size
	return nil
}
func (store *Store) prune(at time.Time, extraBytes int64, extraFiles int) error {
	for _, segment := range append([]*retainedLogSegment(nil), store.segments...) {
		if at.Sub(segment.born) >= store.policy.MaxAge {
			if err := store.remove(segment); err != nil {
				return err
			}
		}
	}
	for store.stats.RetainedBytes+extraBytes > store.policy.MaxBytes || len(store.segments)+1+extraFiles > store.policy.MaxFiles {
		if len(store.segments) == 0 {
			return errors.New("record cannot fit retained budget")
		}
		if err := store.remove(store.segments[0]); err != nil {
			return err
		}
	}
	return nil
}
func (store *Store) fail(err error) error {
	if store.failure == nil {
		store.failure = err
	}
	store.stats.Failed = store.failure != nil
	return err
}

// Append preserves a record within a segment; oversized records are rejected.
// Sink failure remains available independently through Stats and Close.
func (store *Store) Append(stream string, body []byte, at time.Time) (written int, outcome error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.stats.ReceivedBytes += int64(len(body))
	defer func() {
		store.stats.WrittenBytes += int64(written)
		store.stats.LostBytes += int64(len(body) - written)
		if outcome != nil {
			store.fail(outcome)
		}
	}()
	if store.closed {
		return 0, errors.New("log store closed")
	}
	if store.failure != nil {
		return 0, store.failure
	}
	if !validLogStream(stream) || at.IsZero() || at.UnixNano() <= 0 || int64(len(body)) > store.policy.SegmentBytes {
		return 0, errors.New("invalid log record")
	}
	if len(body) == 0 {
		return 0, nil
	}
	if err := store.prune(at, 0, 0); err != nil {
		return 0, err
	}
	segment := store.active[stream]
	if segment != nil && (segment.size+int64(len(body)) > store.policy.SegmentBytes || at.Sub(segment.born) >= store.policy.SegmentAge) {
		if err := segment.file.Close(); err != nil {
			return 0, err
		}
		segment.file = nil
		delete(store.active, stream)
		segment = nil
	}
	extraFiles := 0
	if segment == nil {
		extraFiles = 1
	}
	if err := store.prune(at, int64(len(body)), extraFiles); err != nil {
		return 0, err
	}
	segment = store.active[stream]
	if segment == nil {
		if err := store.prune(at, int64(len(body)), 1); err != nil {
			return 0, err
		}
		if store.next == ^uint64(0) {
			return 0, errors.New("log sequence exhausted")
		}
		store.next++
		name := logSegmentName(store.next, at, stream)
		file, err := store.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
		if err != nil {
			return 0, err
		}
		info, err := file.Stat()
		if err != nil {
			return 0, errors.Join(err, file.Close())
		}
		segment = &retainedLogSegment{name: name, stream: stream, sequence: store.next, born: at, info: info, file: file}
		store.segments = append(store.segments, segment)
		store.active[stream] = segment
	}
	written, outcome = segment.file.Write(body)
	segment.size += int64(written)
	store.stats.RetainedBytes += int64(written)
	if outcome == nil && written != len(body) {
		outcome = io.ErrShortWrite
	}
	return written, outcome
}
func (store *Store) Prune(at time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed {
		return errors.New("log store closed")
	}
	if store.failure != nil {
		return store.failure
	}
	if at.IsZero() || at.UnixNano() <= 0 {
		return store.fail(errors.New("invalid retention time"))
	}
	if err := store.prune(at, 0, 0); err != nil {
		return store.fail(err)
	}
	return nil
}
func (store *Store) Stats() Stats {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := store.stats
	result.Files = len(store.segments) + 1
	// The admitted lock descriptor identifies the filesystem containing these logs.
	// This is whole-filesystem capacity, including other users and reserved space.
	var filesystem syscall.Statfs_t
	if !store.closed && store.lock != nil && syscall.Fstatfs(int(store.lock.Fd()), &filesystem) == nil &&
		filesystem.Bsize > 0 && filesystem.Blocks > 0 && filesystem.Bavail <= filesystem.Blocks &&
		filesystem.Blocks <= uint64((1<<63-1)/filesystem.Bsize) {
		result.FilesystemAvailable = true
		result.FilesystemTotalBytes = int64(filesystem.Blocks) * filesystem.Bsize
		result.FilesystemFreeBytes = int64(filesystem.Bavail) * filesystem.Bsize
	}
	return result
}
func (store *Store) Close() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed {
		return store.failure
	}
	store.closed = true
	for _, segment := range store.segments {
		if segment.file != nil {
			store.failure = errors.Join(store.failure, segment.file.Close())
			segment.file = nil
		}
	}
	if store.lock != nil {
		store.failure = errors.Join(store.failure, syscall.Flock(int(store.lock.Fd()), syscall.LOCK_UN), store.lock.Close())
	}
	if store.root != nil {
		store.failure = errors.Join(store.failure, store.root.Close())
	}
	store.stats.Failed = store.failure != nil
	return store.failure
}
