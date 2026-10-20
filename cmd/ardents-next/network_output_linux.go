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
)

// Borrowed synchronous file bytes. The caller retains ownership through joined
// Close; secret selects clearing of every owned readback copy. No public
// artifact contract carries private bytes.
type networkOutputFile struct {
	name string
	raw  []byte
}

// One finite inventory is written in declared order. observe is a synchronous
// test barrier around actual native effects, never an accepting I/O adapter.
func persistNetworkFiles(ctx context.Context, path string, artifacts []networkOutputFile, secret bool, check func() error, observe func(string, *os.File)) (err error) {
	if len(artifacts) < 1 || len(artifacts) > 129 || !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
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
		artifact networkOutputFile
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
		if secret {
			defer clear(saved)
		}
		n, e := o.file.ReadAt(saved, 0)
		if (e != nil && e != io.EOF) || n != len(o.artifact.raw) || !bytes.Equal(saved[:n], o.artifact.raw) {
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
	// Original input/ancestor reobservation performs physical I/O. Recheck the
	// entire output after that final use; an earlier inventory cannot certify
	// files changed while those original checks were in progress.
	if err = barrier("commit", d); err != nil {
		return err
	}
	return errors.Join(inventory(), ctx.Err())
}
