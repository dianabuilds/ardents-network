//go:build linux

package worker

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

// newAttachment binds an accepted local socket to the exact worker process
// identity whose credentials every read must present.
func newAttachment(connection *net.UnixConn, pid, uid uint32) *Attachment {
	return &Attachment{connection: connection, pid: pid, uid: uid}
}

// inspectActivationSocket pins the on-disk activation socket identity. It is a
// Linux capability step of the portable orchestration in activate.go: the
// root-controlled installed path and the caller-owned 0600 socket inode are
// observed through the installed-path mechanism. macOS and non-Unix adapters
// refuse before this step is reached.
func inspectActivationSocket(inv inventory, path string) (os.FileInfo, error) {
	if path != inv.socket("reader") && path != inv.socket("publisher") || os.Geteuid() == 0 {
		return nil, errors.New("document worker activation address is unavailable")
	}
	if _, err := installedPath(filepath.Dir(path), true); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("document worker activation socket is unavailable")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0600 || stat.Uid != uint32(os.Geteuid()) || stat.Gid != uint32(os.Getegid()) {
		return nil, errors.New("document worker activation socket ownership is invalid")
	}
	return info, nil
}
