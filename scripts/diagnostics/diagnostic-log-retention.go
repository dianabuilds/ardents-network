//go:build ignore

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The private directory is exclusively owned by this store. MaxFiles includes
// its empty lock file. Counters are session-local; retained bytes survive restart.
type logRetentionPolicy struct {
	SegmentBytes int64         `json:"segment_bytes"`
	MaxBytes     int64         `json:"max_bytes"`
	MaxFiles     int           `json:"max_files"`
	SegmentAge   time.Duration `json:"segment_age_ns"`
	MaxAge       time.Duration `json:"max_age_ns"`
}
type logStoreStats struct {
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
type logStore struct {
	mu       sync.Mutex
	root     *os.Root
	lock     *os.File
	policy   logRetentionPolicy
	segments []*retainedLogSegment
	active   map[string]*retainedLogSegment
	next     uint64
	stats    logStoreStats
	failure  error
	closed   bool
}

func validLogStream(stream string) bool {
	switch stream {
	case "stdout", "stderr", "events", "samples":
		return true
	}
	return false
}
func privateLogFile(info os.FileInfo) bool {
	owner, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.Mode().IsRegular() && info.Mode().Perm() == 0600 && owner.Uid == uint32(os.Geteuid()) && owner.Nlink == 1
}
func logSegmentName(sequence uint64, at time.Time, stream string) string {
	return fmt.Sprintf("log-%020d-%020d-%s.log", sequence, at.UnixNano(), stream)
}

func openLogStore(dir string, policy logRetentionPolicy, at time.Time) (_ *logStore, outcome error) {
	if policy.SegmentBytes < 1 || policy.MaxBytes < policy.SegmentBytes || policy.MaxBytes > 1<<30 ||
		policy.MaxFiles < 2 || policy.MaxFiles > 128 || policy.SegmentAge <= 0 || policy.MaxAge < policy.SegmentAge || policy.MaxAge > 365*24*time.Hour ||
		at.IsZero() || at.UnixNano() <= 0 || !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return nil, errors.New("invalid log retention configuration")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(dir))
	if err != nil || parent != filepath.Dir(dir) {
		return nil, errors.New("log parent must be canonical")
	}
	if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || resolved != dir {
		return nil, errors.New("log directory must be canonical")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode().Perm() != 0700 || owner.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("log directory must be owner-private")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	store := &logStore{root: root, policy: policy, active: map[string]*retainedLogSegment{}}
	admitted := false
	defer func() {
		if !admitted {
			outcome = errors.Join(outcome, store.Close())
		}
	}()
	lock, err := root.OpenFile("monitor.lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	store.lock = lock
	lockInfo, err := lock.Stat()
	if err != nil {
		return nil, err
	}
	if !privateLogFile(lockInfo) || lockInfo.Size() != 0 {
		return nil, errors.New("invalid monitoring lock file")
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, errors.New("log store already owned or lock unavailable")
	}
	entries, err := readLogInventory(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Name() == "monitor.lock" {
			continue
		}
		parts := strings.Split(entry.Name(), "-")
		if len(parts) != 4 || parts[0] != "log" || !strings.HasSuffix(parts[3], ".log") {
			return nil, errors.New("unrecognized file in log directory")
		}
		sequence, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil || sequence == 0 {
			return nil, errors.New("invalid log segment sequence")
		}
		ns, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || ns <= 0 {
			return nil, errors.New("invalid log segment time")
		}
		born := time.Unix(0, ns).UTC()
		stream := strings.TrimSuffix(parts[3], ".log")
		if !validLogStream(stream) || logSegmentName(sequence, born, stream) != entry.Name() {
			return nil, errors.New("invalid log segment name")
		}
		info, err := root.Lstat(entry.Name())
		if err != nil {
			return nil, err
		}
		if !privateLogFile(info) || info.Size() > policy.SegmentBytes {
			return nil, errors.New("unsafe log segment")
		}
		segment := &retainedLogSegment{name: entry.Name(), stream: stream, sequence: sequence, born: born, size: info.Size(), info: info}
		store.segments = append(store.segments, segment)
		store.next = max(store.next, sequence)
		store.stats.RetainedBytes += info.Size()
	}
	sort.Slice(store.segments, func(i, j int) bool { return store.segments[i].sequence < store.segments[j].sequence })
	for i := 1; i < len(store.segments); i++ {
		if store.segments[i-1].sequence == store.segments[i].sequence {
			return nil, errors.New("duplicate log sequence")
		}
	}
	if err := store.prune(at, 0, 0); err != nil {
		return nil, err
	}
	admitted = true
	return store, nil
}

func readLogInventory(root *os.Root) (_ []os.DirEntry, outcome error) {
	dir, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { outcome = errors.Join(outcome, dir.Close()) }()
	entries, err := dir.ReadDir(257)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > 256 {
		return nil, errors.New("log inventory exceeds bound")
	}
	return entries, nil
}

func (store *logStore) remove(segment *retainedLogSegment) error {
	info, err := store.root.Lstat(segment.name)
	if err != nil || !privateLogFile(info) || !os.SameFile(segment.info, info) {
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
func (store *logStore) prune(at time.Time, extraBytes int64, extraFiles int) error {
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
func (store *logStore) fail(err error) error {
	if store.failure == nil {
		store.failure = err
	}
	store.stats.Failed = store.failure != nil
	return err
}

// Append preserves a record within a segment; oversized records are rejected.
// Sink failure remains available independently through Stats and Close.
func (store *logStore) Append(stream string, body []byte, at time.Time) (written int, outcome error) {
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
func (store *logStore) Prune(at time.Time) error {
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
func (store *logStore) Stats() logStoreStats {
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
func (store *logStore) Close() error {
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
