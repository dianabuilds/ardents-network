//go:build linux || darwin

package enrollment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// VerifyInfrastructure checks the independent pin before parsing, then the
// seven original bounded files and the actual executable of this process.
// Self-verification does not establish trust in its first execution.
func VerifyInfrastructure(ctx context.Context, root, pin string) (InfrastructureBundle, error) {
	if !canonicalDigest(pin) {
		return InfrastructureBundle{}, ErrInput
	}
	s, err := loadInfrastructure(ctx, root, pin)
	if err != nil {
		return InfrastructureBundle{}, err
	}
	return InfrastructureBundle{initial: s}, nil
}

func ReadInfrastructureCandidate(ctx context.Context, root string) (InfrastructureCandidate, error) {
	s, err := loadInfrastructure(ctx, root, "")
	if err != nil {
		return InfrastructureCandidate{}, err
	}
	return InfrastructureCandidate{candidate: s}, nil
}

func checkInfrastructureDirectory(root *os.Root, invocation string) (err error) {
	d, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, d.Close()) }()
	entries, err := d.ReadDir(9)
	if err != nil && err != io.EOF {
		return err
	}
	if len(entries) != 8 {
		return ErrInventory
	}
	wanted := map[string]bool{InfrastructureInventoryName: true}
	for _, name := range infrastructureNames(invocation) {
		wanted[name] = true
	}
	for _, e := range entries {
		if !wanted[e.Name()] {
			return ErrInventory
		}
	}
	return nil
}

func loadInfrastructure(ctx context.Context, path, pin string) (result *infrastructureSnapshot, err error) {
	if ctx == nil || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, ErrInput
	}
	if pin != "" {
		if err := infrastructurePlatform(); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ancestors := map[string]os.FileInfo{}
	for p := path; ; p = filepath.Dir(p) {
		info, e := os.Lstat(p)
		if e != nil {
			return nil, e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrInventory
		}
		ancestors[p] = info
		if filepath.Dir(p) == p {
			break
		}
	}
	original, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !original.IsDir() || original.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInventory
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, root.Close(), ctx.Err())
		if err != nil {
			result = nil
		}
	}()
	opened, err := root.Lstat(".")
	if err != nil {
		return nil, err
	}
	if !os.SameFile(original, opened) {
		return nil, ErrBinding
	}
	read := func(name string, limit int64) ([]byte, os.FileInfo, error) {
		before, e := root.Lstat(name)
		if e != nil {
			return nil, nil, e
		}
		raw, after, e := readBundleFile(ctx, root, name, limit)
		if e != nil {
			return nil, nil, e
		}
		if !sameInfrastructureFile(before, after) {
			return nil, nil, ErrBinding
		}
		return raw, after, nil
	}
	raw, info, err := read(InfrastructureInventoryName, 16<<10)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	observedPin := hex.EncodeToString(sum[:])
	if pin != "" && pin != observedPin {
		return nil, ErrPin
	}
	d, err := parseInfrastructureInventory(raw)
	if err != nil {
		return nil, err
	}
	if err := checkInfrastructureDirectory(root, d.Invocation); err != nil {
		return nil, err
	}
	files := map[string][]byte{InfrastructureInventoryName: raw}
	identities := map[string]os.FileInfo{InfrastructureInventoryName: info}
	for _, name := range infrastructureNames(d.Invocation) {
		body, fi, err := read(name, infrastructureLimit(name))
		if err != nil {
			return nil, err
		}
		if !matchesDigest(body, d.Files[name]) {
			return nil, ErrBinding
		}
		files[name], identities[name] = body, fi
	}
	if pin != "" {
		if err := verifyInfrastructureProcess(ctx, identities["ardents-node-linux-amd64"], files["ardents-node-linux-amd64"]); err != nil {
			return nil, err
		}
	}
	if err := checkInfrastructureDirectory(root, d.Invocation); err != nil {
		return nil, err
	}
	for name, before := range identities {
		after, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}
		if !sameInfrastructureFile(before, after) {
			return nil, ErrBinding
		}
		if err := verifyOwnedFile(after); err != nil {
			return nil, err
		}
	}
	after, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(original, after) || original.Mode() != after.Mode() {
		return nil, ErrBinding
	}
	if pin != "" {
		if err := verifyInfrastructureProcess(ctx, identities["ardents-node-linux-amd64"], files["ardents-node-linux-amd64"]); err != nil {
			return nil, err
		}
	}
	for name, before := range ancestors {
		after, e := os.Lstat(name)
		if e != nil {
			return nil, e
		}
		if !os.SameFile(before, after) || before.Mode() != after.Mode() {
			return nil, ErrBinding
		}
	}
	return &infrastructureSnapshot{InfrastructureFacts{d.Platform, d.ReleaseIdentity, d.ReleaseVersion, d.Invocation}, files, observedPin, ctx}, nil
}
