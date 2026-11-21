//go:build linux

package node

import (
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// credential retains the exact runtime TLS material through Route join. It
// grants no provisioning, offline signing or membership and never hot-reloads.
type credential struct {
	certificate tls.Certificate
	key         ed25519.PrivateKey
}

func loadCredential(certPath, keyPath string) (_ *credential, result error) {
	if !filepath.IsAbs(certPath) || !filepath.IsAbs(keyPath) || filepath.Clean(certPath) != certPath || filepath.Clean(keyPath) != keyPath {
		return nil, errors.New("invalid Node credential paths")
	}
	cert, err := readCredentialFile(certPath, false)
	if err != nil {
		return nil, err
	}
	key, err := readCredentialFile(keyPath, true)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	pair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	private, ok := pair.PrivateKey.(ed25519.PrivateKey)
	if !ok || len(private) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid Node runtime TLS key")
	}
	return &credential{certificate: pair, key: private}, nil
}

func readCredentialFile(path string, private bool) (raw []byte, result error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		result = errors.Join(result, f.Close())
		if result != nil {
			clear(raw)
			raw = nil
		}
	}()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.Join(errors.New("invalid Node credential file"), err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	protected := private && protectedCredentialAccess(info, uint32(os.Geteuid()), uint32(os.Getegid()))
	if private && (!ok || !protected && (stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0077 != 0)) {
		return nil, errors.New("invalid Node TLS key ownership")
	}
	if protected {
		if err := protectedCredentialAncestors(path); err != nil {
			return nil, err
		}
	}
	raw, err = io.ReadAll(io.LimitReader(f, 8193))
	if err != nil || len(raw) > 8192 {
		clear(raw)
		return nil, errors.Join(errors.New("runtime Node credential unavailable"), err)
	}
	if protected {
		after, statErr := f.Stat()
		named, nameErr := os.Lstat(path)
		if statErr != nil || nameErr != nil || !sameProtectedCredential(info, after) || !sameProtectedCredential(info, named) {
			clear(raw)
			return nil, errors.Join(errors.New("original protected Node key changed"), statErr, nameErr)
		}
		if err := protectedCredentialAncestors(path); err != nil {
			clear(raw)
			return nil, err
		}
	}
	return raw, nil
}

// This finite native file layout grants no installed Startup or Node duty. The
// actual system principal can read root's non-writable group key. Installation
// independently admits its fixed account and exclusive group before effects;
// this physical reader grants neither that proof nor any installed permission.
// The ordinary runtime-owned private-file policy remains independently checked.
func protectedCredentialAccess(info os.FileInfo, uid, gid uint32) bool {
	if info == nil || uid < 100 || uid > 999 || gid != uid || !info.Mode().IsRegular() || info.Mode()&os.ModeType != 0 || info.Mode().Perm() != 0640 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == gid && native.Nlink == 1
}

func protectedCredentialAncestors(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return errors.New("protected Node credential path unavailable")
	}
	for directory := filepath.Dir(path); ; directory = filepath.Dir(directory) {
		info, err := os.Lstat(directory)
		if err != nil {
			return err
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || info.Mode().Perm()&0022 != 0 || native.Uid != 0 || native.Gid != 0 {
			return errors.New("protected Node credential ancestor unavailable")
		}
		if directory == "/" {
			return nil
		}
	}
}

func sameProtectedCredential(before, after os.FileInfo) bool {
	if before == nil || after == nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return false
	}
	a, okA := before.Sys().(*syscall.Stat_t)
	b, okB := after.Sys().(*syscall.Stat_t)
	return okA && okB && a.Uid == b.Uid && a.Gid == b.Gid && a.Nlink == b.Nlink && a.Ctim == b.Ctim
}

func (key *credential) close() error {
	if key != nil {
		clear(key.key)
		key.certificate.PrivateKey = nil
	}
	return nil
}
