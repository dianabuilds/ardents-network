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
	if private && (!ok || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0077 != 0) {
		return nil, errors.New("invalid Node TLS key ownership")
	}
	raw, err = io.ReadAll(io.LimitReader(f, 8193))
	if err != nil || len(raw) > 8192 {
		clear(raw)
		return nil, errors.Join(errors.New("runtime Node credential unavailable"), err)
	}
	return raw, nil
}

func (key *credential) close() error {
	if key != nil {
		clear(key.key)
		key.certificate.PrivateKey = nil
	}
	return nil
}
