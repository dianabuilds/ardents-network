//go:build !windows

package alpha

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestPersistentFloorRootMustBelongToCurrentUser(t *testing.T) {
	otherUser := uint32(os.Geteuid()) ^ 1
	info := persistentFloorPermissionInfo{mode: os.ModeDir | 0o700, stat: &syscall.Stat_t{Uid: otherUser}}

	if err := validatePersistentFloorRootPermissions("unused", info); err == nil {
		t.Fatal("foreign-owned root was accepted")
	}
}

type persistentFloorPermissionInfo struct {
	mode os.FileMode
	stat *syscall.Stat_t
}

func (info persistentFloorPermissionInfo) Name() string       { return "floor" }
func (info persistentFloorPermissionInfo) Size() int64        { return 0 }
func (info persistentFloorPermissionInfo) Mode() os.FileMode  { return info.mode }
func (info persistentFloorPermissionInfo) ModTime() time.Time { return time.Time{} }
func (info persistentFloorPermissionInfo) IsDir() bool        { return info.mode.IsDir() }
func (info persistentFloorPermissionInfo) Sys() any           { return info.stat }
