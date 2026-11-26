//go:build linux

package artifacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/pathcheck"
)

// PlatformInputs retains native observations for one stopped or pre-start
// configuration operation. The caller owns the lifetime and must close it.
type PlatformInputs struct {
	inputs      []*ProtectedInput
	directories []*DirectoryChain
	resources   []*platformInputFile
}

type platformInputFile struct {
	parent *DirectoryChain
	file   *os.File
	info   os.FileInfo
	raw    []byte
}

// ReadRuntime pins the role's configuration input under /etc/ardents.
// Public trust inputs are root-owned. Private service material excludes other
// users and retains its original owner/inode/bytes. All ancestors are root-only
// writable, and all these paths are outside the unit's runtime write grants.
func (f *PlatformInputs) ReadRuntime(path string, maximum int64, private bool) ([]byte, error) {
	if !pathcheck.LiteralPOSIXPath(path) || !strings.HasPrefix(path, "/etc/ardents/") {
		return nil, ErrInput
	}
	p, err := OpenDirectoryChain(context.Background(), filepath.Dir(path), nil)
	if err != nil {
		return nil, err
	}
	input := &platformInputFile{parent: p}
	f.resources = append(f.resources, input)
	if err := p.CheckRootAccess(); err != nil {
		return nil, err
	}
	input.file, err = p.OpenFile(filepath.Base(path))
	if err != nil {
		return nil, err
	}
	input.info, input.raw, err = ReadRetainedFile(input.file, maximum, func(mode os.FileMode, owner uint32) bool {
		if mode != mode.Perm() || mode.Perm()&0022 != 0 {
			return false
		}
		if !private {
			return owner == 0
		}
		return mode.Perm()&0077 == 0 && (owner == 0 || owner >= 100 && owner <= 999)
	}, nil)
	if err != nil || input.parent.CheckRetainedFile(input.file, input.info, input.raw, nil) != nil {
		return nil, ErrInput
	}
	return input.raw, nil
}

func (f *PlatformInputs) RetainDirectory(path string) error {
	if !pathcheck.LiteralPOSIXPath(path) {
		return ErrInput
	}
	p, err := OpenDirectoryChain(context.Background(), path, nil)
	if err != nil {
		return err
	}
	f.directories = append(f.directories, p)
	return p.CheckRootAccess()
}

func (f *PlatformInputs) Read(path string, maximum int64, executable bool) ([]byte, error) {
	if !pathcheck.LiteralPOSIXPath(path) {
		return nil, ErrInput
	}
	input, err := OpenProtectedInput(context.Background(), path, maximum, false)
	if err != nil {
		return nil, err
	}
	f.inputs = append(f.inputs, input)
	if executable && input.Info().Mode().Perm() != 0555 {
		return nil, ErrInput
	}
	return input.Bytes(), nil
}

// ReadAccountFiles retains the original passwd and group files for the
// process owner to interpret.
func (f *PlatformInputs) ReadAccountFiles() ([]byte, []byte, error) {
	passwd, err := f.Read("/etc/passwd", 1<<20, false)
	if err != nil {
		return nil, nil, err
	}
	group, err := f.Read("/etc/group", 1<<20, false)
	if err != nil {
		return nil, nil, ErrInput
	}
	return passwd, group, nil
}

func (f *PlatformInputs) Check(ctx context.Context) error {
	var result error
	for _, input := range f.inputs {
		result = errors.Join(result, input.Check(ctx))
	}
	for _, p := range f.directories {
		result = errors.Join(result, p.CheckRootAccess())
	}
	for _, input := range f.resources {
		result = errors.Join(result, input.parent.CheckRootAccess(), input.parent.CheckRetainedFile(input.file, input.info, input.raw, nil))
	}
	return errors.Join(result, ctx.Err())
}

func (f *PlatformInputs) Close() error {
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

// CheckRunningExecutable compares the actual mapped executable identity and
// bytes with the original root-controlled enrolled artifact.
func CheckRunningExecutable(path string, wanted []byte) (result error) {
	if !pathcheck.LiteralPOSIXPath(path) {
		return ErrInput
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
	if err != nil || !os.SameFile(actual, original) || !ProtectedFileMode(original, false) {
		return ErrInput
	}
	raw, err := io.ReadAll(io.LimitReader(self, 64<<20+1))
	if err != nil || !bytes.Equal(raw, wanted) {
		return ErrInput
	}
	return nil
}
