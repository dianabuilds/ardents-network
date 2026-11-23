//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/platform/process"
)

// Sequential pre-start composition retains existing native file owners. There
// is no live runtime root, shared lease, journal, observer goroutine or mutex.
type platformFiles struct {
	inputs      []*artifactio.ProtectedInput
	directories []*preparationPath
	resources   []*sourceInputFile
}

// Reuse command-local original-file custody, not a Source runtime lifetime.
// Public trust inputs are root-owned; private service material excludes other
// users and retains its original owner/inode/bytes. All ancestors are root-only
// writable, and all these paths are outside the unit's runtime write grants.
func (f *platformFiles) ReadRuntime(path string, maximum int64, private bool) ([]byte, error) {
	if !platformBelow("/etc/ardents", path) {
		return nil, errPlatformInput
	}
	p, err := openPreparationParent(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	input := &sourceInputFile{parent: p}
	f.resources = append(f.resources, input)
	if err := signingAncestors(p); err != nil {
		return nil, err
	}
	input.file, err = p.OpenFile(filepath.Base(path))
	if err != nil {
		return nil, err
	}
	input.info, err = input.file.Stat()
	if err != nil || !input.info.Mode().IsRegular() || input.info.Size() < 1 || input.info.Size() > maximum {
		return nil, errPlatformInput
	}
	stat, ok := input.info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 || input.info.Mode() != input.info.Mode().Perm() || input.info.Mode().Perm()&0022 != 0 || (!private && stat.Uid != 0) || (private && (input.info.Mode().Perm()&0077 != 0 || stat.Uid != 0 && (stat.Uid < 100 || stat.Uid > 999))) {
		return nil, errPlatformInput
	}
	input.raw, err = io.ReadAll(io.LimitReader(input.file, maximum+1))
	if err != nil || int64(len(input.raw)) != input.info.Size() || input.check() != nil {
		return nil, errPlatformInput
	}
	return input.raw, nil
}

func (f *platformFiles) RetainDirectory(path string) error {
	if !platformPath(path) {
		return errPlatformInput
	}
	p, err := openPreparationParent(path)
	if err != nil {
		return err
	}
	f.directories = append(f.directories, p)
	return signingAncestors(p)
}

func (f *platformFiles) Read(path string, maximum int64, executable bool) ([]byte, error) {
	if !platformPath(path) {
		return nil, errPlatformInput
	}
	input, err := artifactio.OpenProtectedInput(context.Background(), path, maximum, false)
	if err != nil {
		return nil, err
	}
	f.inputs = append(f.inputs, input)
	if executable && input.Info().Mode().Perm() != 0555 {
		return nil, errPlatformInput
	}
	return input.Bytes(), nil
}

func (f *platformFiles) readAccount(name string) (uint32, error) {
	passwd, err := f.Read("/etc/passwd", 1<<20, false)
	if err != nil {
		return 0, err
	}
	group, err := f.Read("/etc/group", 1<<20, false)
	if err != nil {
		return 0, errPlatformInput
	}
	uid, ok := process.DedicatedAccountID(passwd, group, name)
	if !ok {
		return 0, errPlatformInput
	}
	return uid, nil
}

func (f *platformFiles) Check(ctx context.Context) error {
	var result error
	for _, input := range f.inputs {
		result = errors.Join(result, input.Check(ctx))
	}
	for _, p := range f.directories {
		result = errors.Join(result, signingAncestors(p))
	}
	for _, input := range f.resources {
		result = errors.Join(result, signingAncestors(input.parent), input.check())
	}
	return errors.Join(result, ctx.Err())
}

func (f *platformFiles) Close() error {
	var result error
	for i := len(f.resources) - 1; i >= 0; i-- {
		input := f.resources[i]
		clear(input.raw)
		if input.file != nil {
			result = errors.Join(result, input.file.Close())
		}
		result = errors.Join(result, input.parent.Close())
	}
	f.resources = nil
	for i := len(f.inputs) - 1; i >= 0; i-- {
		result = errors.Join(result, f.inputs[i].Close())
	}
	f.inputs = nil
	for i := len(f.directories) - 1; i >= 0; i-- {
		result = errors.Join(result, f.directories[i].Close())
	}
	f.directories = nil
	return result
}

func platformOwnExecutable(path string, wanted []byte) (result error) {
	if !platformPath(path) {
		return errPlatformInput
	}
	self, err := os.Open("/proc/self/exe")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, self.Close()) }()
	actual, err := self.Stat()
	if err != nil {
		return err
	}
	original, err := os.Lstat(path)
	if err != nil || !os.SameFile(actual, original) || !artifactio.ProtectedFileMode(original, false) {
		return errPlatformInput
	}
	raw, err := io.ReadAll(io.LimitReader(self, 64<<20+1))
	if err != nil || !bytes.Equal(raw, wanted) {
		return errPlatformInput
	}
	return nil
}
