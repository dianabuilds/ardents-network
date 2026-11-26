package launch

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/generation"
	"github.com/dianabuilds/ardents-network/internal/domain/release"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// StagePlatformGeneration seals a fresh Endpoint generation against the actual
// platform-prepared account and empty mutable roots. It installs no system
// resources and never resumes or overwrites an earlier staging attempt.
func StagePlatformGeneration(ctx context.Context, request Request, authorization Authorization, verifier *release.Verifier) error {
	if ctx == nil || os.Geteuid() != 0 || request.declared == nil || verifier == nil {
		return ErrInput
	}
	if err := observeRequestCustody(ctx, request); err != nil {
		return err
	}
	facts, initial := authorization.InitialFacts()
	if !initial || facts.ManifestSHA256 != request.ManifestSHA256() {
		return ErrAuthorization
	}
	uid, gid, err := observeEndpointAccount()
	if err != nil || uid != gid || uid < 100 || uid > 999 {
		return errors.Join(ErrNativeUnavailable, err)
	}
	prepared := preparedInstallation{uid: uid, gid: gid}
	for _, path := range mutableRoots(request.declared.Headless) {
		identity, err := observeVacantPlatformRoot(path, uid, gid)
		if err != nil {
			return err
		}
		prepared.roots = append(prepared.roots, identity)
	}
	return sealPlatformGeneration(ctx, request, authorization, verifier, prepared, func() error {
		for _, before := range prepared.roots {
			after, err := observeVacantPlatformRoot(before.Path, uid, gid)
			if err != nil || after != before {
				return errors.Join(ErrBinding, err)
			}
		}
		return nil
	})
}

func sealPlatformGeneration(ctx context.Context, request Request, authorization Authorization, verifier *release.Verifier, prepared preparedInstallation, observeRoots func() error) (returnedErr error) {
	program, descriptor := authorization.Targets()
	gid := prepared.gid
	directory := request.declared.InstallationRoot
	ancestors, err := rootDirectoryAncestors(filepath.Dir(directory))
	if err != nil {
		return err
	}
	files, selection, err := assembleGeneration(ctx, request, authorization, prepared)
	if err != nil {
		return err
	}
	frozen, binding, err := freezeGenerationInventory(request, directory, files, selection, gid)
	if err != nil {
		return err
	}
	reader := &installedFiles{mutableDirectories: make(map[string]os.FileInfo)}
	check := func() error {
		for path, before := range ancestors {
			after, err := os.Lstat(path)
			if err != nil || !sameObservedDirectory(before, after) {
				return errors.Join(ErrBinding, err)
			}
		}
		if err := observeRoots(); err != nil {
			return err
		}
		return errors.Join(observeRequestCustody(ctx, request),
			reader.observeAccountAndRoots(inspectedGeneration{binding: binding}),
			verifier.CheckAuthorizations(ctx, program, descriptor), ctx.Err())
	}
	if err := check(); err != nil {
		return err
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, root.Close()) }()
	handle, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, handle.Close()) }()
	original, err := handle.Stat()
	if err != nil || !privateJournalDirectory(original) {
		return errors.Join(ErrBinding, err)
	}
	if err := syncDirectDirectory(filepath.Dir(directory)); err != nil {
		return err
	}
	if err := root.Mkdir("generations", 0700); err != nil {
		return err
	}
	parent, err := root.OpenFile("generations", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, parent.Close()) }()
	parentInfo, err := parent.Stat()
	if err != nil || !privateJournalDirectory(parentInfo) {
		return errors.Join(ErrBinding, err)
	}
	owned, err := generation.Create(ctx, filepath.Join(directory, "generations"), parentInfo, selection.GenerationDigest, gid)
	if owned != nil {
		defer func() { returnedErr = errors.Join(returnedErr, owned.Close()) }()
	}
	if err != nil {
		return err
	}
	for _, name := range generation.Names() {
		if _, err := owned.CreateFile(ctx, name); err != nil {
			return err
		}
		if err := owned.Write(ctx, name, frozen[name]); err != nil {
			return err
		}
	}
	if err := owned.Seal(ctx); err != nil {
		return err
	}
	if err := owned.Close(); err != nil {
		return err
	}
	owned = nil
	if err := check(); err != nil {
		return err
	}
	body, err := canonicalJSON(selection)
	if err != nil {
		return err
	}
	// Selection remains inaccessible to the participant until the original
	// complete generation, its parent and the selection bytes are durable.
	if _, err := writeStagedFile(ctx, root, "selection.json", body, 0640, gid); err != nil {
		return err
	}
	if err := parent.Chown(0, int(gid)); err != nil {
		return err
	}
	if err := parent.Chmod(0750); err != nil {
		return err
	}
	if err := parent.Sync(); err != nil {
		return err
	}
	if err := handle.Sync(); err != nil {
		return err
	}
	current, err := os.Lstat(directory)
	if err != nil || !sameObservedDirectory(original, current) {
		return errors.Join(ErrBinding, err)
	}
	if err := check(); err != nil {
		return err
	}
	if err := handle.Chown(0, int(gid)); err != nil {
		return err
	}
	if err := handle.Chmod(0750); err != nil {
		return err
	}
	return errors.Join(handle.Sync(), syncDirectDirectory(filepath.Dir(directory)), ctx.Err())
}

func observeVacantPlatformRoot(path string, uid, gid uint32) (result rootIdentity, returnedErr error) {
	before, err := os.Lstat(path)
	if err != nil || !matchesMutableDirectory(before, uid, gid) {
		return rootIdentity{}, errors.Join(ErrBinding, err)
	}
	file, err := os.Open(path)
	if err != nil {
		return rootIdentity{}, err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	current, err := file.Stat()
	if err != nil || !os.SameFile(before, current) || !matchesMutableDirectory(current, uid, gid) {
		return rootIdentity{}, errors.Join(ErrBinding, err)
	}
	if _, err := file.Readdirnames(1); !errors.Is(err, io.EOF) {
		return rootIdentity{}, errors.Join(ErrBinding, err)
	}
	native := current.Sys().(*syscall.Stat_t)
	return rootIdentity{Path: path, Device: uint64(native.Dev), Inode: native.Ino}, nil
}
