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
	var admitted *initialNetworkSigning
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
	operation, err := admitInitialNetworkSigning(args[0], plan.raw, prepared.raw, args[3])
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
	key, err := parseInitialNetworkKey(secret.raw)
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
	return &networkSigningResult{digest: sha256.Sum256(artifacts[0].raw), interval: operation.initialSigningInterval}, err
}

// One finite inventory is written in declared order. observe is a synchronous
// test barrier around actual native effects, never an accepting I/O adapter.
func persistNetworkSigning(ctx context.Context, path string, artifacts []networkSignedArtifact, check func() error, observe func(string, *os.File)) (err error) {
	if len(artifacts) < 1 || len(artifacts) > 129 || path == "/" {
		return errNetworkSigning
	}
	seen := map[string]bool{}
	for _, a := range artifacts {
		if filepath.Base(a.name) != a.name || a.name == "." || len(a.raw) == 0 || len(a.raw) > maximumNetworkPreparationPlan || seen[a.name] {
			return errNetworkSigning
		}
		seen[a.name] = true
	}
	p, err := openPreparationParent(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, p.close()) }()
	info, err := p.parent().Stat()
	if err != nil || !preparationPrivate(info, true) || info.Mode() != os.ModeDir|0700 {
		return errors.Join(errNetworkSigning, err)
	}
	var filesystem syscall.Statfs_t
	if err = syscall.Fstatfs(int(p.parent().Fd()), &filesystem); err != nil || filesystem.Type != 0xef53 {
		return errors.Join(errNetworkSigning, err)
	}
	if err = errors.Join(signingAncestors(p), p.parent().Sync(), check(), ctx.Err()); err != nil {
		return err
	}
	if err = syscall.Mkdirat(int(p.parent().Fd()), filepath.Base(path), 0700); err != nil {
		return err
	}
	d, err := openPreparationAt(p.parent(), filepath.Base(path), syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	di, err := d.Stat()
	if err != nil {
		return errors.Join(err, d.Close())
	}
	p.directories = append(p.directories, preparationDirectory{file: d, name: filepath.Base(path), info: di})
	if !preparationPrivate(di, true) || di.Mode() != os.ModeDir|0700 {
		return errNetworkSigning
	}
	barrier := func(label string, f *os.File) error {
		if observe != nil {
			observe(label, f)
		}
		return errors.Join(signingAncestors(p), check(), ctx.Err())
	}
	if err = errors.Join(barrier("directory-created", d), p.directories[len(p.directories)-2].file.Sync()); err != nil {
		return err
	}
	// Retain read descriptors until the complete inventory and every barrier
	// have succeeded; original output inodes cannot be reopened and adopted.
	type output struct {
		artifact networkSignedArtifact
		file     *os.File
		info     os.FileInfo
	}
	var outputs []output
	defer func() {
		for _, o := range outputs {
			err = errors.Join(err, o.file.Close())
		}
	}()
	verify := func(o output) error {
		info, e := o.file.Stat()
		if e != nil || !os.SameFile(o.info, info) || !signingFileAllowed(info, true) {
			return errors.Join(errNetworkSigning, e)
		}
		saved := make([]byte, len(o.artifact.raw)+1)
		n, e := o.file.ReadAt(saved, 0)
		if (e != nil && e != io.EOF) || !bytes.Equal(saved[:n], o.artifact.raw) {
			return errors.Join(errNetworkSigning, e)
		}
		link, e := openPreparationAt(d, o.artifact.name, syscall.O_RDONLY, 0)
		if e != nil {
			return e
		}
		linked, se := link.Stat()
		e = errors.Join(se, link.Close())
		if e != nil || !os.SameFile(o.info, linked) || !signingFileAllowed(linked, true) {
			return errors.Join(errNetworkSigning, e)
		}
		return nil
	}
	for _, a := range artifacts {
		if err = barrier("before-file", d); err != nil {
			return err
		}
		f, e := openPreparationAt(d, a.name, syscall.O_RDWR|syscall.O_CREAT|syscall.O_EXCL, 0600)
		if e != nil {
			return e
		}
		original, e := f.Stat()
		if e != nil || !signingFileAllowed(original, true) {
			return errors.Join(errNetworkSigning, e, f.Close())
		}
		if e = barrier("file-created", f); e != nil {
			return errors.Join(e, f.Close())
		}
		n, we := f.Write(a.raw)
		if we == nil && n != len(a.raw) {
			we = io.ErrShortWrite
		}
		be := barrier("written", f)
		se := f.Sync()
		// Capture a matching original reader before joining the writer Close.
		r, re := openPreparationAt(d, a.name, syscall.O_RDONLY, 0)
		ce := f.Close()
		if r != nil {
			outputs = append(outputs, output{a, r, original})
		}
		err = errors.Join(we, be, se, re, ce, barrier("closed", d))
		if err != nil {
			return err
		}
		if err = verify(outputs[len(outputs)-1]); err != nil {
			return err
		}
	}
	inventory := func() error {
		if _, e := d.Seek(0, io.SeekStart); e != nil {
			return e
		}
		names, e := d.Readdirnames(len(artifacts) + 1)
		if (e != nil && e != io.EOF) || len(names) != len(artifacts) {
			return errors.Join(errNetworkSigning, e)
		}
		for _, name := range names {
			if !seen[name] {
				return errNetworkSigning
			}
		}
		for _, o := range outputs {
			if e = verify(o); e != nil {
				return e
			}
		}
		return nil
	}
	if err = errors.Join(inventory(), barrier("before-directory-sync", d)); err != nil {
		return err
	}
	if observe != nil {
		observe("directory-sync", d)
	}
	if err = errors.Join(d.Sync(), p.directories[len(p.directories)-2].file.Sync(), barrier("synced", d)); err != nil {
		return err
	}
	return errors.Join(inventory(), barrier("commit", d))
}
