package installation

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Only the native reader can construct this physical provenance. It is not
// serialized, and checked declarations cannot become privileged input.
type requestCustody struct {
	path     string
	digest   [32]byte
	identity os.FileInfo
}

func readOwnedRequest(ctx context.Context, filename string, initial bool) (result Request, returnedErr error) {
	if os.Geteuid() != 0 || !canonicalPath(filename) || filepath.Base(filename) == "/" {
		return Request{}, ErrNativeUnavailable
	}
	parent := filepath.Dir(filename)
	ancestors, err := rootDirectoryAncestors(parent)
	if err != nil {
		return Request{}, err
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return Request{}, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, root.Close(), ctx.Err())
		if returnedErr != nil {
			result = Request{}
		}
	}()
	name := filepath.Base(filename)
	original, err := root.Lstat(name)
	if err != nil || !ownedRequestFile(original) || original.Size() == 0 || original.Size() > 64<<10 {
		return Request{}, errors.Join(ErrNativeUnavailable, err)
	}
	file, err := root.Open(name)
	if err != nil {
		return Request{}, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, file.Close())
		if returnedErr != nil {
			result = Request{}
		}
	}()
	opened, err := file.Stat()
	if err != nil || !sameRequestFile(original, opened) {
		return Request{}, errors.Join(ErrNativeUnavailable, err)
	}
	body, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil || len(body) > 64<<10 || int64(len(body)) != original.Size() {
		return Request{}, errors.Join(ErrNativeUnavailable, err)
	}
	request, err := DecodeRequest(ctx, body, initial)
	if err != nil {
		return Request{}, err
	}
	finalHandle, handleErr := file.Stat()
	finalPath, pathErr := root.Lstat(name)
	if err := errors.Join(handleErr, pathErr); err != nil ||
		!sameRequestFile(original, finalHandle) || !sameRequestFile(original, finalPath) {
		return Request{}, errors.Join(ErrNativeUnavailable, err)
	}
	for ancestor, before := range ancestors {
		after, err := os.Lstat(ancestor)
		if err != nil || !rootDirectory(after) || !os.SameFile(before, after) {
			return Request{}, errors.Join(ErrNativeUnavailable, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return Request{}, err
	}
	request.custody = &requestCustody{path: filename, digest: sha256.Sum256(body), identity: original}
	return request, nil
}

func ownedRequestFile(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0 && native.Nlink == 1
}

func sameRequestFile(before, after os.FileInfo) bool {
	if !ownedRequestFile(before) || !ownedRequestFile(after) || !os.SameFile(before, after) ||
		before.Size() != after.Size() || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		return false
	}
	left, leftOK := before.Sys().(*syscall.Stat_t)
	right, rightOK := after.Sys().(*syscall.Stat_t)
	return leftOK && rightOK && left.Ctim == right.Ctim
}

func rootDirectory(info os.FileInfo) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0
}

func rootDirectoryAncestors(directory string) (map[string]os.FileInfo, error) {
	if !canonicalPath(directory) {
		return nil, ErrNativeUnavailable
	}
	observed := make(map[string]os.FileInfo)
	for {
		info, err := os.Lstat(directory)
		if err != nil || !rootDirectory(info) {
			return nil, errors.Join(ErrNativeUnavailable, err)
		}
		observed[directory] = info
		parent := filepath.Dir(directory)
		if parent == directory {
			return observed, nil
		}
		directory = parent
	}
}
