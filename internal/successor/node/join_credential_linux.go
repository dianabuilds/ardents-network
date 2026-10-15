//go:build linux

package node

import (
	"bytes"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// joinCredential retains original file custody as well as the exact in-memory
// TLS key. A changed path never authorizes reloading or replacing that key.
type joinCredential struct {
	mu             sync.Mutex
	material       *credential
	files          [2]*os.File
	identity       [2]os.FileInfo
	contents       [2][]byte
	failed, result error
	closed         bool
}

func loadJoinCredential(certPath, keyPath string) (_ *joinCredential, result error) {
	if !filepath.IsAbs(certPath) || !filepath.IsAbs(keyPath) || filepath.Clean(certPath) != certPath || filepath.Clean(keyPath) != keyPath {
		return nil, errors.New("invalid original JOIN credential paths")
	}
	c := &joinCredential{}
	defer func() {
		if result != nil {
			result = errors.Join(result, c.close())
		}
	}()
	for i, path := range []string{certPath, keyPath} {
		// The ordinary credential reader validates canonical paths, ownership,
		// permissions and bounded regular input before original pins are acquired.
		raw, err := readCredentialFile(path, i == 1)
		if err != nil {
			return nil, err
		}
		c.contents[i] = raw
		c.files[i], err = os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		c.identity[i], err = c.files[i].Stat()
		if err != nil {
			return nil, err
		}
		info := c.identity[i]
		stat, native := info.Sys().(*syscall.Stat_t)
		if !info.Mode().IsRegular() || info.Size() > 8192 || i == 1 && (!native || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0077 != 0) {
			return nil, errors.New("original JOIN credential pin unavailable")
		}
	}
	pair, err := tls.X509KeyPair(c.contents[0], c.contents[1])
	if err != nil {
		return nil, err
	}
	key, ok := pair.PrivateKey.(ed25519.PrivateKey)
	if !ok || len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("original JOIN TLS key unavailable")
	}
	c.material = &credential{certificate: pair, key: key}
	if err := c.check(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *joinCredential) check() error {
	if c == nil {
		return errors.New("original JOIN credential absent")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failed != nil {
		return c.failed
	}
	if c.closed {
		return errors.New("original JOIN credential closed")
	}
	for i, f := range c.files {
		info, err := f.Stat()
		if err == nil {
			var pathInfo os.FileInfo
			pathInfo, err = os.Lstat(f.Name())
			if err == nil && (!info.Mode().IsRegular() || !os.SameFile(c.identity[i], info) || !os.SameFile(info, pathInfo) || info.Mode() != c.identity[i].Mode() || info.Size() != int64(len(c.contents[i]))) {
				err = errors.New("original JOIN credential custody changed")
			}
		}
		if err == nil {
			original, originalOK := c.identity[i].Sys().(*syscall.Stat_t)
			current, currentOK := info.Sys().(*syscall.Stat_t)
			if !originalOK || !currentOK || original.Uid != current.Uid || original.Gid != current.Gid || original.Ctim != current.Ctim {
				err = errors.New("original JOIN credential metadata changed")
			}
		}
		if err == nil {
			raw := make([]byte, len(c.contents[i])+1)
			n, readErr := f.ReadAt(raw, 0)
			if readErr != nil && readErr != io.EOF {
				err = readErr
			}
			if err == nil && !bytes.Equal(raw[:n], c.contents[i]) {
				err = errors.New("original JOIN credential bytes changed")
			}
			clear(raw)
		}
		if err != nil {
			c.failed = err
			return err
		}
	}
	return nil
}

func (c *joinCredential) close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		for i, f := range c.files {
			if f != nil {
				c.result = errors.Join(c.result, f.Close())
			}
			clear(c.contents[i])
		}
		c.result = errors.Join(c.result, c.material.close())
	}
	return c.result
}
