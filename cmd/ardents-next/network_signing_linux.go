//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// signingInput retains the original file and every direct ancestor. A later
// lookup compares with this inode; it cannot adopt a newly supplied input.
type signingInput struct {
	parent  *preparationPath
	file    *os.File
	name    string
	info    os.FileInfo
	raw     []byte
	private bool
	failure error
}

func signingAncestors(p *preparationPath) error {
	if err := p.check(); err != nil {
		return err
	}
	for _, d := range p.directories {
		info, err := d.file.Stat()
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != 0 || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
			return errNetworkSigning
		}
	}
	return nil
}

func signingFileAllowed(info os.FileInfo, private bool) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == 0 && stat.Nlink == 1 && info.Mode().IsRegular() &&
		info.Mode() == info.Mode().Perm() && info.Mode().Perm()&0022 == 0 && (!private || info.Mode() == 0600)
}

func openSigningInput(ctx context.Context, path string, maximum int64, private bool) (input *signingInput, err error) {
	p, err := openPreparationParent(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	input = &signingInput{parent: p, name: filepath.Base(path), private: private}
	defer func() {
		if err != nil {
			err = errors.Join(err, input.close())
			input = nil
		}
	}()
	if err = errors.Join(ctx.Err(), signingAncestors(p)); err != nil {
		return input, err
	}
	input.file, err = openPreparationAt(p.parent(), input.name, syscall.O_RDONLY, 0)
	if err != nil {
		return input, err
	}
	input.info, err = input.file.Stat()
	if err != nil || !signingFileAllowed(input.info, private) || input.info.Size() < 1 || input.info.Size() > maximum {
		return input, errors.Join(errNetworkSigning, err)
	}
	input.raw, err = io.ReadAll(io.LimitReader(input.file, maximum+1))
	if err != nil || int64(len(input.raw)) > maximum {
		return input, errors.Join(errNetworkSigning, err)
	}
	return input, input.check(ctx)
}

func (s *signingInput) close() (err error) {
	// Physical reads are synchronous and joined before owned copies are erased.
	if s.private {
		clear(s.raw)
	}
	if s.file != nil {
		err = s.file.Close()
	}
	return errors.Join(err, s.parent.close())
}

func (s *signingInput) check(ctx context.Context) (err error) {
	if s.failure != nil {
		return s.failure
	}
	defer func() { s.failure = err }()
	if err := errors.Join(ctx.Err(), signingAncestors(s.parent)); err != nil {
		return err
	}
	info, err := s.file.Stat()
	if err != nil {
		return err
	}
	initial, ok1 := s.info.Sys().(*syscall.Stat_t)
	current, ok2 := info.Sys().(*syscall.Stat_t)
	if !ok1 || !ok2 || !os.SameFile(s.info, info) || !signingFileAllowed(info, s.private) ||
		initial.Ctim != current.Ctim || initial.Mtim != current.Mtim || initial.Mode != current.Mode || initial.Gid != current.Gid || info.Size() != s.info.Size() {
		return errNetworkSigning
	}
	linked, err := openPreparationAt(s.parent.parent(), s.name, syscall.O_RDONLY, 0)
	if err != nil {
		return err
	}
	observed, statErr := linked.Stat()
	err = errors.Join(statErr, linked.Close())
	if err != nil || !os.SameFile(s.info, observed) {
		return errors.Join(errNetworkSigning, err)
	}
	// Re-read only the retained original descriptor. Detect byte changes even
	// when an adversarial writer restores the original size and timestamps.
	saved := make([]byte, len(s.raw)+1)
	if s.private {
		defer clear(saved)
	}
	n, readErr := s.file.ReadAt(saved, 0)
	if readErr != nil && readErr != io.EOF {
		return readErr
	}
	if n != len(s.raw) || !bytes.Equal(saved[:n], s.raw) {
		return errNetworkSigning
	}
	after, err := s.file.Stat()
	if err != nil {
		return err
	}
	last, ok := after.Sys().(*syscall.Stat_t)
	if !ok || last.Ctim != initial.Ctim || last.Mtim != initial.Mtim || !signingFileAllowed(after, s.private) {
		return errNetworkSigning
	}
	return ctx.Err()
}

func executeNetworkSigning(ctx context.Context, args []string) (*networkSigningResult, error) {
	return executeObservedNetworkSigning(ctx, args, nil)
}

func executeObservedNetworkSigning(ctx context.Context, args []string, observe func(string, *os.File)) (result *networkSigningResult, err error) {
	if os.Geteuid() != 0 {
		return nil, errNetworkSigning
	}
	var admitted *networkSigning
	defer func() {
		err = errors.Join(err, ctx.Err())
		if admitted != nil {
			err = errors.Join(err, admitted.checkTime(time.Now()))
		}
	}()
	plan, err := openSigningInput(ctx, args[1], maximumNetworkPreparationPlan, false)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, plan.close()) }()
	prepared, err := openSigningInput(ctx, args[2], maximumNetworkPreparationPlan, false)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, prepared.close()) }()
	operation, err := admitNetworkSigning(args[0], plan.raw, prepared.raw, args[3])
	if err != nil {
		return nil, err
	}
	admitted = operation
	if err = operation.checkTime(time.Now()); err != nil {
		return nil, err
	}
	secret, err := openSigningInput(ctx, args[4], 16<<10, true)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, secret.close()) }()
	key, err := parseNetworkSigningKey(secret.raw)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	if observe != nil {
		observe("inputs-read", secret.file)
	}
	check := func() error {
		return errors.Join(plan.check(ctx), prepared.check(ctx), secret.check(ctx), operation.checkTime(time.Now()), ctx.Err())
	}
	if err = check(); err != nil {
		return nil, err
	}
	artifacts, err := operation.sign(key, time.Now())
	if err != nil {
		return nil, err
	}
	if err = check(); err != nil {
		return nil, err
	}
	if observe != nil {
		observe("signed", secret.file)
	}
	err = persistNetworkSigning(ctx, args[5], artifacts, check, observe)
	return &networkSigningResult{digest: sha256.Sum256(artifacts[0].raw), interval: operation.networkSigningInterval}, err
}

// Public signed artifacts borrow the native finite-file mechanism without
// granting it signing or semantic authority.
func persistNetworkSigning(ctx context.Context, path string, artifacts []networkSignedArtifact, check func() error, observe func(string, *os.File)) error {
	files := make([]networkOutputFile, 0, len(artifacts))
	for _, a := range artifacts {
		files = append(files, networkOutputFile{a.name, a.raw})
	}
	return persistNetworkFiles(ctx, path, files, false, check, observe)
}
