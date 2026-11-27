//go:build linux

package enrollment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
	"github.com/dianabuilds/ardents-network/internal/domain/enrollment/infrastructureinventory"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
)

// InfrastructureFacts are copied declarations. The native Bundle retains
// separate independent-pin and actual running-program provenance.
type InfrastructureFacts = infrastructureinventory.Facts

type infrastructureSnapshot struct {
	facts  InfrastructureFacts
	files  map[string][]byte
	pin    string
	caller context.Context
}

// InfrastructureBundle has private construction and cannot be reconstructed
// from a public receipt, facts, candidate or supplied boolean.
type InfrastructureBundle struct{ initial *infrastructureSnapshot }

// InfrastructureCandidate supplies a frozen inventory without initial trust.
type InfrastructureCandidate struct{ candidate *infrastructureSnapshot }

// Check retains the original loader's cancellation through later handoff.
// Physical input descriptors were joined before this immutable result existed.
func (b InfrastructureBundle) Check() error    { return b.initial.check() }
func (c InfrastructureCandidate) Check() error { return c.candidate.check() }
func (s *infrastructureSnapshot) check() error {
	if s == nil || s.caller == nil {
		return ErrInput
	}
	if !matchesDigest(s.files[infrastructureinventory.Name], s.pin) {
		return ErrBinding
	}
	return s.caller.Err()
}

func (b InfrastructureBundle) Facts() (InfrastructureFacts, bool)    { return b.initial.factsCopy() }
func (c InfrastructureCandidate) Facts() (InfrastructureFacts, bool) { return c.candidate.factsCopy() }
func (b InfrastructureBundle) File(name string) ([]byte, bool)       { return b.initial.file(name) }
func (c InfrastructureCandidate) File(name string) ([]byte, bool)    { return c.candidate.file(name) }
func (s *infrastructureSnapshot) factsCopy() (InfrastructureFacts, bool) {
	if s == nil {
		return InfrastructureFacts{}, false
	}
	return s.facts, true
}
func (s *infrastructureSnapshot) file(name string) ([]byte, bool) {
	if s == nil {
		return nil, false
	}
	raw, ok := s.files[name]
	return bytes.Clone(raw), ok
}

// VerifyInfrastructure checks the independent pin before parsing, then the
// seven original bounded files and the actual executable of this process.
// Self-verification does not establish trust in its first execution.
func VerifyInfrastructure(ctx context.Context, root, pin string) (InfrastructureBundle, error) {
	defer measurement.Begin(runtimeevent.EnrollmentVerify).End()

	if !canonicaldigest.Valid(pin) {
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

func checkInfrastructureDirectory(root *os.Root, companions []infrastructureinventory.File) (err error) {
	d, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, d.Close()) }()
	entries, err := d.ReadDir(len(companions) + 2)
	if err != nil && err != io.EOF {
		return err
	}
	if len(entries) != len(companions)+1 {
		return ErrInventory
	}
	wanted := map[string]bool{infrastructureinventory.Name: true}
	for _, file := range companions {
		wanted[file.Name] = true
	}
	for _, e := range entries {
		if !wanted[e.Name()] {
			return ErrInventory
		}
	}
	return nil
}

func loadInfrastructure(ctx context.Context, path, pin string) (result *infrastructureSnapshot, err error) {
	if ctx == nil || !pathcheck.IsCleanAbsolute(path) {
		return nil, ErrInput
	}
	if err := infrastructurePlatform(); err != nil {
		return nil, err
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
	raw, info, err := read(infrastructureinventory.Name, infrastructureinventory.MaximumSize)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	observedPin := hex.EncodeToString(sum[:])
	if pin != "" && pin != observedPin {
		return nil, ErrPin
	}
	facts, digests, err := infrastructureinventory.Parse(raw)
	if err != nil {
		return nil, ErrInventory
	}
	companions := infrastructureinventory.Files(facts.Invocation)
	if err := checkInfrastructureDirectory(root, companions); err != nil {
		return nil, err
	}
	files := map[string][]byte{infrastructureinventory.Name: raw}
	identities := map[string]os.FileInfo{infrastructureinventory.Name: info}
	for _, file := range companions {
		body, fi, err := read(file.Name, file.MaximumSize)
		if err != nil {
			return nil, err
		}
		if !matchesDigest(body, digests[file.Name]) {
			return nil, ErrBinding
		}
		files[file.Name], identities[file.Name] = body, fi
	}
	if pin != "" {
		if err := verifyInfrastructureProcess(ctx, identities[infrastructureinventory.ProgramName], files[infrastructureinventory.ProgramName]); err != nil {
			return nil, err
		}
	}
	if err := checkInfrastructureDirectory(root, companions); err != nil {
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
		if err := verifyInfrastructureProcess(ctx, identities[infrastructureinventory.ProgramName], files[infrastructureinventory.ProgramName]); err != nil {
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
	return &infrastructureSnapshot{facts, files, observedPin, ctx}, nil
}
func sameInfrastructureFile(a, b os.FileInfo) bool {
	if !sameFileState(a, b) {
		return false
	}
	x, ok := a.Sys().(*syscall.Stat_t)
	y, yes := b.Sys().(*syscall.Stat_t)
	return ok && yes && x.Ctim == y.Ctim && x.Uid == y.Uid && x.Gid == y.Gid && x.Nlink == 1 && y.Nlink == 1
}
