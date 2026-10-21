package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
)

// sourceSelection owns independent read-only original files and ancestors.
// Its caller serializes observation and joins all uses before close. No writer
// lease, initial provenance, Release proof or process admission crosses here.
type sourceSelection struct {
	ctx        context.Context
	binding    sourceBinding
	sealed     string
	files      map[string]*sourceFile
	dirs       map[string]*os.File
	identities map[string]os.FileInfo
	closed     bool
	terminal   error
	// Test-only hold after real file I/O; it cannot supply observations.
	observed func(*os.File)
}

func sourceGenerationNames() []string {
	return []string{ProgramName, TemplateName, "binding.json", "infrastructure-enrollment.json", DescriptorName, "rendered-source.service", "root.json", "snapshot.json", "source-plan.json", "source-request.json", "targets.json", "timestamp.json"}
}

func openSourceSelection(ctx context.Context, bindingPath string) (result *sourceSelection, err error) {
	if ctx == nil || ctx.Err() != nil || !sourcePath(bindingPath) || filepath.Base(bindingPath) != "binding.json" {
		return nil, ErrInput
	}
	s := &sourceSelection{ctx: ctx, files: map[string]*sourceFile{}, dirs: map[string]*os.File{}, identities: map[string]os.FileInfo{}}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.close())
			result = nil
		}
	}()
	raw, err := s.read(bindingPath, 32<<10, 0644, 0, 0)
	if err != nil {
		return nil, err
	}
	s.binding, err = decodeSourceBinding(raw)
	if err != nil {
		return nil, err
	}
	b := s.binding
	if bindingPath != filepath.Join(b.Request.InstallationRoot, "binding.json") ||
		(os.Geteuid() != 0 && (uint32(os.Geteuid()) != b.UID || uint32(os.Getegid()) != b.GID)) {
		return nil, ErrBinding
	}
	s.sealed = filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	for _, d := range []struct {
		path     string
		mode     os.FileMode
		uid, gid uint32
	}{
		{b.Request.InstallationRoot, 0755, 0, 0}, {filepath.Dir(s.sealed), 0755, 0, 0}, {s.sealed, 0555, 0, 0},
		{b.Request.CredentialDirectory, 0755, 0, 0}, {b.Request.StateRoot, 0700, b.UID, b.GID}, {b.Request.ParticipationRoot, 0700, b.UID, b.GID},
	} {
		if err = s.pinDirectory(d.path, d.mode, d.uid, d.gid); err != nil {
			return nil, err
		}
	}
	for _, name := range sourceGenerationNames() {
		limit, mode := int64(1<<20), os.FileMode(0444)
		switch name {
		case ProgramName:
			limit, mode = 64<<20, 0555
		case "binding.json":
			limit = 32 << 10
		case TemplateName, DescriptorName, "infrastructure-enrollment.json", "rendered-source.service", "source-plan.json", "source-request.json":
			limit = 16 << 10
		}
		if _, err = s.read(filepath.Join(s.sealed, name), limit, mode, 0, 0); err != nil {
			return nil, err
		}
	}
	if err = s.checkInventory(); err != nil {
		return nil, err
	}
	get := func(name string) []byte { return s.files[filepath.Join(s.sealed, name)].raw }
	if !bytes.Equal(raw, get("binding.json")) || digest(get("source-request.json")) != b.RequestSHA256 ||
		digest(get(ProgramName)) != b.ProgramSHA256 || digest(get(TemplateName)) != b.TemplateSHA256 ||
		digest(get("infrastructure-enrollment.json")) != b.InventorySHA256 || digest(get("targets.json")) != b.TargetsSHA256 ||
		digest(get("source-plan.json")) != b.PlanSHA256 || digest(get("rendered-source.service")) != b.UnitSHA256 ||
		digest(get(DescriptorName)) != b.Generation {
		return nil, ErrBinding
	}
	if err = b.Request.checkPlan(get("source-plan.json")); err != nil {
		return nil, err
	}
	d, err := ValidateResources(get(DescriptorName), get(ProgramName), get(TemplateName))
	if err != nil {
		return nil, err
	}
	public := map[string][]byte{}
	for _, name := range []string{ProgramName, TemplateName, DescriptorName, "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
		public[name] = get(name)
	}
	inventory, err := enrollment.PrepareInfrastructureInventory(enrollment.InfrastructureFacts{Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: d.ReleaseVersion, Invocation: d.Invocation}, public)
	if err != nil || !bytes.Equal(inventory, get("infrastructure-enrollment.json")) {
		return nil, errors.Join(ErrBinding, err)
	}
	for _, c := range []struct {
		name     string
		maximum  int64
		mode     os.FileMode
		uid, gid uint32
	}{
		{"certificate.pem", 64 << 10, 0444, 0, 0}, {"private-key.pem", 16 << 10, 0600, b.UID, b.GID}, {"client-ca.pem", 64 << 10, 0444, 0, 0},
	} {
		if _, err = s.read(filepath.Join(b.Request.CredentialDirectory, c.name), c.maximum, c.mode, c.uid, c.gid); err != nil {
			return nil, err
		}
	}
	if err = checkSourceRuntimePair(s.files[filepath.Join(b.Request.CredentialDirectory, "certificate.pem")].raw, s.files[filepath.Join(b.Request.CredentialDirectory, "private-key.pem")].raw); err != nil {
		return nil, err
	}
	unit, err := s.read("/etc/systemd/system/ardents-source.service", 16<<10, 0644, 0, 0)
	if err != nil || !bytes.Equal(unit, get("rendered-source.service")) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err = s.pinParents(b.Request.Clock); err != nil {
		return nil, err
	}
	return s, s.observe()
}

func (s *sourceSelection) pinParents(path string) error {
	for p := filepath.Dir(path); ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil || !trustedDirectory(info) {
			return errors.Join(ErrBinding, err)
		}
		if err = s.pinDirectory(p, info.Mode().Perm(), 0, 0); err != nil {
			return err
		}
		if p == "/" {
			return nil
		}
	}
}

func (s *sourceSelection) pinDirectory(path string, mode os.FileMode, uid, gid uint32) error {
	if original, ok := s.identities[path]; ok {
		now, err := os.Lstat(path)
		if err != nil || !os.SameFile(original, now) || !sourceAccess(original, mode, uid, gid, true) || !sourceAccess(now, mode, uid, gid, true) {
			return errors.Join(ErrBinding, err)
		}
		return nil
	}
	f, info, err := openSourceDirectory(path, mode, uid, gid)
	if f != nil {
		s.dirs[path] = f
	}
	if err != nil {
		return err
	}
	s.identities[path] = info
	return s.ctx.Err()
}

func (s *sourceSelection) read(path string, limit int64, mode os.FileMode, uid, gid uint32) ([]byte, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.pinParents(path); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	leaf := &sourceFile{path: path, file: f}
	s.files[path] = leaf
	leaf.info, err = f.Stat()
	if err != nil || !sourceAccess(leaf.info, mode, uid, gid, false) || leaf.info.Size() <= 0 || leaf.info.Size() > limit {
		return nil, errors.Join(ErrBinding, err)
	}
	var fs syscall.Statfs_t
	if err = syscall.Fstatfs(int(f.Fd()), &fs); err != nil || fs.Type != 0xef53 {
		return nil, errors.Join(ErrBinding, err)
	}
	leaf.raw, err = io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(leaf.raw)) != leaf.info.Size() {
		return nil, errors.Join(ErrBinding, err)
	}
	return leaf.raw, s.ctx.Err()
}

func (s *sourceSelection) checkInventory() error {
	names := sourceGenerationNames()
	slices.Sort(names)
	return checkSourceInventory(s.sealed, names)
}

func (s *sourceSelection) observe() (err error) {
	if s == nil || s.ctx == nil || s.closed {
		return ErrInput
	}
	if s.terminal != nil {
		return s.terminal
	}
	defer func() {
		if cause := s.ctx.Err(); cause != nil && err != cause {
			err = errors.Join(err, cause)
		}
		if err != nil {
			s.terminal = err
		}
	}()
	if err = s.ctx.Err(); err != nil {
		return err
	}
	checkPaths := func() error {
		for path, original := range s.identities {
			if e := observeSourceDirectory(path, s.dirs[path], original); e != nil {
				return e
			}
		}
		for path, f := range s.files {
			now, e := f.file.Stat()
			named, ne := os.Lstat(path)
			if e != nil || ne != nil || !sameSourceFile(f.info, now) || !sameSourceFile(f.info, named) {
				return errors.Join(ErrBinding, e, ne)
			}
		}
		clock, e := os.Lstat(s.binding.Request.Clock)
		if e != nil || !sourceAccess(clock, 0644, 0, 0, false) {
			return errors.Join(ErrBinding, e)
		}
		return s.checkInventory()
	}
	if err = checkPaths(); err != nil {
		return err
	}
	for _, f := range s.files {
		if err = s.ctx.Err(); err != nil {
			return err
		}
		raw := make([]byte, len(f.raw)+1)
		n, e := f.file.ReadAt(raw, 0)
		if s.observed != nil {
			s.observed(f.file)
		}
		equal := bytes.Equal(raw[:n], f.raw)
		clear(raw)
		if (e != nil && e != io.EOF) || !equal {
			return errors.Join(ErrBinding, e)
		}
	}
	if err = checkPaths(); err != nil {
		return err
	}
	return s.ctx.Err()
}

func (s *sourceSelection) close() error {
	if s == nil {
		return nil
	}
	if !s.closed {
		s.closed = true
		for _, f := range s.files {
			s.terminal = errors.Join(s.terminal, f.file.Close())
			clear(f.raw)
		}
		for _, f := range s.dirs {
			s.terminal = errors.Join(s.terminal, f.Close())
		}
	}
	return s.terminal
}
