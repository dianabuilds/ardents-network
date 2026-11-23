//go:build linux

package artifacts

import (
	"crypto/tls"
	"errors"
	"io"
	"os"
	"syscall"
)

const ownerKeyPairFileLimit = 8 << 10

// ReadOwnerKeyPair reads the caller's validated TLS configuration paths.
// It admits the private key only through the effective user's original opened
// descriptor, without sharing the installed Node's protected-group policy.
func ReadOwnerKeyPair(certPath, keyPath string) (tls.Certificate, error) {
	cert, err := ReadBoundedFile(certPath, ownerKeyPairFileLimit)
	if err != nil {
		return tls.Certificate{}, err
	}
	key, err := readOwnerPrivateKey(keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	defer clear(key)
	return tls.X509KeyPair(cert, key)
}

func readOwnerPrivateKey(path string) (key []byte, result error) {
	// Nonblocking/no-follow open cannot stall on a substituted FIFO or follow
	// a substituted symlink; authority is checked on the opened descriptor.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("route private key unavailable")
	}
	defer func() {
		result = errors.Join(result, f.Close())
		if result != nil {
			clear(key)
			key = nil
		}
	}()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("route private key permissions invalid")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("route private key owner differs")
	}
	raw, err := io.ReadAll(io.LimitReader(f, ownerKeyPairFileLimit+1))
	if err != nil || len(raw) > ownerKeyPairFileLimit {
		clear(raw)
		return nil, errors.New("route private key unavailable")
	}
	return raw, nil
}
