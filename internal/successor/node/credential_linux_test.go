package node

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// Detached native metadata tests only the finite file-access grammar. It is
// never passed to the real reader or used as account, Startup or Node authority.
type credentialMetadata struct {
	mode   os.FileMode
	native syscall.Stat_t
}

func (m credentialMetadata) Name() string       { return "key" }
func (m credentialMetadata) Size() int64        { return 1 }
func (m credentialMetadata) Mode() os.FileMode  { return m.mode }
func (m credentialMetadata) ModTime() time.Time { return time.Unix(1, 0) }
func (m credentialMetadata) IsDir() bool        { return m.mode.IsDir() }
func (m credentialMetadata) Sys() any           { return &m.native }

func TestProtectedNodeCredentialAccessRequiresExactRootGroupFile(t *testing.T) {
	good := credentialMetadata{mode: 0640, native: syscall.Stat_t{Uid: 0, Gid: 988, Nlink: 1}}
	if !protectedCredentialAccess(good, 988, 988) {
		t.Fatal("exact root-controlled system-group layout refused")
	}
	for _, ids := range [][2]uint32{{0, 0}, {99, 99}, {1000, 1000}, {988, 989}, {989, 989}} {
		if protectedCredentialAccess(good, ids[0], ids[1]) {
			t.Fatal("foreign principal admitted root-controlled key", ids)
		}
	}
	for _, mode := range []os.FileMode{0600, 0644, 0660, 0440, 0640 | os.ModeSymlink, 0640 | os.ModeDir, 0640 | os.ModeSetuid, 0640 | os.ModeSetgid} {
		other := good
		other.mode = mode
		if protectedCredentialAccess(other, 988, 988) {
			t.Fatal("foreign root key access accepted", mode)
		}
	}
	for _, mutate := range []func(*syscall.Stat_t){func(s *syscall.Stat_t) { s.Uid = 988 }, func(s *syscall.Stat_t) { s.Gid = 989 }, func(s *syscall.Stat_t) { s.Nlink = 2 }} {
		other := good
		mutate(&other.native)
		if protectedCredentialAccess(other, 988, 988) {
			t.Fatal("foreign owner/group/hardlink accepted")
		}
	}
}
