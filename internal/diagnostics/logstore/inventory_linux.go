package logstore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func validLogStream(stream string) bool {
	switch stream {
	case "stdout", "stderr", "events", "samples":
		return true
	}
	return false
}

// IsPrivateFile checks the effective user's private single-link regular file.
func IsPrivateFile(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.Mode().IsRegular() && info.Mode().Perm() == 0600 && owner.Uid == uint32(os.Geteuid()) && owner.Nlink == 1
}
func logSegmentName(sequence uint64, at time.Time, stream string) string {
	return fmt.Sprintf("log-%020d-%020d-%s.log", sequence, at.UnixNano(), stream)
}

// Open retains the owner-private directory and acquires its sole writer lease.
// It verifies the bounded inventory before publishing the original store.
func Open(dir string, policy Policy, at time.Time) (_ *Store, outcome error) {
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
	store := &Store{root: root, policy: policy, active: map[string]*retainedLogSegment{}}
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
	if !IsPrivateFile(lockInfo) || lockInfo.Size() != 0 {
		return nil, errors.New("invalid monitoring lock file")
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, errors.New("log store already owned or lock unavailable")
	}
	entries, err := ReadInventory(root)
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
		if !IsPrivateFile(info) || info.Size() > policy.SegmentBytes {
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

// ReadInventory reads one bounded directory inventory through the caller's
// original root. Owners still validate each entry against their own grammar.
func ReadInventory(root *os.Root) (_ []os.DirEntry, outcome error) {
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
