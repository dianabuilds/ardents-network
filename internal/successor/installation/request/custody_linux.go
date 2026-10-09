package request

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Only the native reader can construct this physical provenance. It is not
// serialized, and checked declarations cannot become privileged input.
type Origin struct {
	path     string
	digest   [32]byte
	identity os.FileInfo
}

func readOwned(ctx context.Context, filename string, initial bool) (result Document, returnedErr error) {
	if os.Geteuid() != 0 || !canonicalPath(filename) || filepath.Base(filename) == "/" {
		return Document{}, ErrNativeUnavailable
	}
	parent := filepath.Dir(filename)
	ancestors, err := rootDirectoryAncestors(parent)
	if err != nil {
		return Document{}, err
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return Document{}, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, root.Close(), ctx.Err())
		if returnedErr != nil {
			result = Document{}
		}
	}()
	name := filepath.Base(filename)
	original, err := root.Lstat(name)
	if err != nil || !ownedRequestFile(original) || original.Size() == 0 || original.Size() > 64<<10 {
		return Document{}, errors.Join(ErrNativeUnavailable, err)
	}
	file, err := root.Open(name)
	if err != nil {
		return Document{}, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, file.Close())
		if returnedErr != nil {
			result = Document{}
		}
	}()
	opened, err := file.Stat()
	if err != nil || !sameRequestFile(original, opened) {
		return Document{}, errors.Join(ErrNativeUnavailable, err)
	}
	body, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil || len(body) > 64<<10 || int64(len(body)) != original.Size() {
		return Document{}, errors.Join(ErrNativeUnavailable, err)
	}
	request, err := Decode(ctx, body, initial)
	if err != nil {
		return Document{}, err
	}
	finalHandle, handleErr := file.Stat()
	finalPath, pathErr := root.Lstat(name)
	if err := errors.Join(handleErr, pathErr); err != nil ||
		!sameRequestFile(original, finalHandle) || !sameRequestFile(original, finalPath) {
		return Document{}, errors.Join(ErrNativeUnavailable, err)
	}
	for ancestor, before := range ancestors {
		after, err := os.Lstat(ancestor)
		if err != nil || !rootDirectory(after) || !os.SameFile(before, after) {
			return Document{}, errors.Join(ErrNativeUnavailable, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	request.custody = &Origin{path: filename, digest: sha256.Sum256(body), identity: original}
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

// Observe independently rereads the original root-owned input and checks its
// exact declarations, bytes and native identity. It grants no effect authority.
func (origin *Origin) Observe(ctx context.Context, declared Declaration) error {
	if ctx == nil || origin == nil {
		return ErrInput
	}
	fresh, err := ReadOwned(ctx, origin.path, declared.ManifestSHA256 != "")
	if err != nil {
		return err
	}
	wanted, err := marshalDeclaration(declared)
	if err != nil || fresh.custody == nil || fresh.custody.digest != origin.digest ||
		sha256.Sum256(wanted) != origin.digest || !sameRequestFile(origin.identity, fresh.custody.identity) {
		return errors.Join(ErrChanged, err)
	}
	return ctx.Err()
}

func marshalDeclaration(value Declaration) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}
