package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// sourceHistory owns only independent read-only directory observations. Release
// retains its own exclusive lease and all floor policy. Its caller serializes
// checks and joins them before close; no live root crosses the owner boundary.
type sourceHistory struct {
	ctx        context.Context
	path       string
	dirs       map[string]*os.File
	identities map[string]os.FileInfo
	terminal   error
	closed     bool
	// Test-only hold after an actual original Release observation.
	observed func()
}

func openSourceHistory(ctx context.Context, v *release.Verifier, path string) (result *sourceHistory, err error) {
	if ctx == nil || v == nil || !sourcePath(path) || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	h := &sourceHistory{ctx: ctx, path: path, dirs: map[string]*os.File{}, identities: map[string]os.FileInfo{}}
	defer func() {
		if err != nil {
			err = errors.Join(err, h.close())
			result = nil
		}
	}()
	for p := path; ; p = filepath.Dir(p) {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		info, e := os.Lstat(p)
		if e != nil || !trustedDirectory(info) || p == path && info.Mode().Perm()&0077 != 0 {
			return nil, errors.Join(ErrBinding, e)
		}
		f, original, e := openSourceDirectory(p, info.Mode().Perm(), 0, 0)
		if f != nil {
			h.dirs[p] = f
		}
		if e != nil {
			return nil, e
		}
		h.identities[p] = original
		if p == "/" {
			break
		}
	}
	var fs syscall.Statfs_t
	if err = syscall.Fstatfs(int(h.dirs[path].Fd()), &fs); err != nil || fs.Type != 0xef53 {
		return nil, errors.Join(ErrBinding, err)
	}
	return h, h.check(v)
}

func (h *sourceHistory) check(v *release.Verifier) (err error) {
	if h == nil || h.ctx == nil || v == nil {
		return ErrInput
	}
	if h.terminal != nil {
		return h.terminal
	}
	if h.closed {
		return ErrInput
	}
	defer func() {
		err = errors.Join(err, h.ctx.Err())
		if err != nil {
			h.terminal = err
		}
	}()
	if err = h.ctx.Err(); err != nil {
		return err
	}
	paths := func() error {
		for path, original := range h.identities {
			if err := observeSourceDirectory(path, h.dirs[path], original); err != nil {
				return err
			}
		}
		return nil
	}
	if err = paths(); err != nil {
		return err
	}
	if err = v.CheckHistoryPath(h.ctx, h.path); err != nil {
		return err
	}
	if h.observed != nil {
		h.observed()
	}
	return paths()
}

// The private admitted-record commitment binds the initial selection to its
// original retained history. It is not a public receipt or Release permission.
// Existing stopped-only journals with a different commitment remain retained;
// they cannot acquire first-start authority by supplying another history.
func (h *sourceHistory) commitment(v *release.Verifier, binding []byte) (string, error) {
	if _, err := decodeSourceBinding(binding); err != nil {
		return "", err
	}
	return h.commitmentBytes(v, binding, "ardents-source-initial-history-v1")
}

// A replacement has a separate private history commitment. This accepts only
// its complete immutable grammar, never an initial binding with a predecessor
// inserted. Original history custody and Release floors remain independently
// checked; the digest does not supply a join, authorization or runtime permit.
func (h *sourceHistory) selectedCommitment(v *release.Verifier, binding []byte) (string, error) {
	if _, err := decodeSourceBinding(binding); err == nil {
		return h.commitment(v, binding)
	}
	if _, err := decodeSourceReplacementBinding(binding); err != nil {
		return "", err
	}
	return h.commitmentBytes(v, binding, "ardents-source-replacement-history-v1")
}

func (h *sourceHistory) commitmentBytes(v *release.Verifier, binding []byte, schema string) (string, error) {
	if err := h.check(v); err != nil {
		return "", err
	}
	native := h.identities[h.path].Sys().(*syscall.Stat_t)
	raw, err := json.Marshal(struct {
		Schema        string `json:"schema"`
		BindingSHA256 string `json:"binding_sha256"`
		History       string `json:"history"`
		Device        uint64 `json:"device"`
		Inode         uint64 `json:"inode"`
		Mode          uint32 `json:"mode"`
		UID           uint32 `json:"uid"`
		GID           uint32 `json:"gid"`
	}{schema, digest(binding), h.path, uint64(native.Dev), native.Ino,
		uint32(h.identities[h.path].Mode().Perm()), native.Uid, native.Gid})
	if err != nil {
		return "", err
	}
	return digest(append(raw, '\n')), nil
}

func (h *sourceHistory) close() error {
	if h == nil {
		return nil
	}
	if !h.closed {
		h.closed = true
		for _, f := range h.dirs {
			h.terminal = errors.Join(h.terminal, f.Close())
		}
		h.terminal = errors.Join(h.terminal, h.ctx.Err())
	}
	return h.terminal
}
