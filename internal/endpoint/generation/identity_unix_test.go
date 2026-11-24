//go:build linux || darwin

package generation

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Detached native metadata tests only the finite access grammar that does not
// compare inode identity. It is never passed to the real generation owner or
// used as Installation, Release or startup authority.
type identityMetadata struct {
	mode   os.FileMode
	size   int64
	native syscall.Stat_t
}

func (m identityMetadata) Name() string       { return "record" }
func (m identityMetadata) Size() int64        { return m.size }
func (m identityMetadata) Mode() os.FileMode  { return m.mode }
func (m identityMetadata) ModTime() time.Time { return time.Unix(1, 0) }
func (m identityMetadata) IsDir() bool        { return m.mode.IsDir() }
func (m identityMetadata) Sys() any           { return &m.native }

func TestOwnedFileRequiresExactRootSingleLinkRegular(t *testing.T) {
	good := identityMetadata{mode: 0600, native: syscall.Stat_t{Uid: 0, Gid: 0, Nlink: 1}}
	if !ownedFile(good) {
		t.Fatal("exact root-owned single-link regular file refused")
	}
	for _, mode := range []os.FileMode{0640, 0644, 0440} {
		other := good
		other.mode = mode
		if !ownedFile(other) {
			t.Fatal("read-only group/other access refused", mode)
		}
	}
	for _, mode := range []os.FileMode{0660, 0646, 0666, 0600 | os.ModeSymlink, 0600 | os.ModeDir} {
		other := good
		other.mode = mode
		if ownedFile(other) {
			t.Fatal("writable or non-regular file admitted", mode)
		}
	}
	for _, mutate := range []func(*syscall.Stat_t){func(s *syscall.Stat_t) { s.Uid = 988 }, func(s *syscall.Stat_t) { s.Gid = 988 }, func(s *syscall.Stat_t) { s.Nlink = 2 }} {
		other := good
		mutate(&other.native)
		if ownedFile(other) {
			t.Fatal("foreign owner/group/hardlink admitted")
		}
	}
	if ownedFile(nil) {
		t.Fatal("absent observation admitted")
	}
}

func TestSealedDirectoryRequiresRootGroupAndClosedAccess(t *testing.T) {
	good := identityMetadata{mode: os.ModeDir | 0750, native: syscall.Stat_t{Uid: 0, Gid: 988, Nlink: 2}}
	if !sealedDirectory(good, 988) {
		t.Fatal("exact sealed root:group directory refused")
	}
	for _, mode := range []os.FileMode{os.ModeDir | 0700, os.ModeDir | 0755, os.ModeDir | 0770, 0750} {
		other := good
		other.mode = mode
		if sealedDirectory(other, 988) {
			t.Fatal("foreign sealed access admitted", mode)
		}
	}
	for _, mutate := range []func(*syscall.Stat_t){func(s *syscall.Stat_t) { s.Uid = 988 }, func(s *syscall.Stat_t) { s.Gid = 989 }} {
		other := good
		mutate(&other.native)
		if sealedDirectory(other, 988) {
			t.Fatal("foreign sealed owner/group admitted")
		}
	}
	if sealedDirectory(nil, 988) {
		t.Fatal("absent directory admitted")
	}
}

// os.SameFile only admits os.Stat results, so the inode-identity predicates run
// against real files. The root-custody positive path executes under the Linux
// native installation profile; on an ordinary host the same predicate must
// refuse the non-root owner.
func TestFileObservationMatchesBindsRootGroupAccessAndChangeTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record")
	if err := os.WriteFile(path, []byte("abc"), 0640); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := fileObservation{identity: info, body: []byte("abc"), gid: info.Sys().(*syscall.Stat_t).Gid, mode: 0640}
	matches := fileObservationMatches(expected, info)
	if want := os.Geteuid() == 0; matches != want {
		t.Fatalf("root-custody observation: matches=%v euid=%d", matches, os.Geteuid())
	}
	if snapshotFileMatches(expected, info) != matches {
		t.Fatal("sealed observation diverged from the base identity")
	}
	foreign := expected
	foreign.gid = expected.gid + 1
	if fileObservationMatches(foreign, info) || snapshotFileMatches(foreign, info) {
		t.Fatal("foreign group admitted")
	}
	short := expected
	short.body = []byte("ab")
	if fileObservationMatches(short, info) {
		t.Fatal("foreign size admitted")
	}
	if fileObservationMatches(fileObservation{}, info) || fileObservationMatches(expected, nil) {
		t.Fatal("absent observation admitted")
	}
}

func TestSameDirectoryRequiresRootCustody(t *testing.T) {
	info, err := os.Lstat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	matches := sameDirectory(info, info)
	if want := os.Geteuid() == 0; matches != want {
		t.Fatalf("root-custody directory: matches=%v euid=%d", matches, os.Geteuid())
	}
	if sameDirectory(info, nil) || sameDirectory(nil, info) {
		t.Fatal("absent directory admitted")
	}
}
