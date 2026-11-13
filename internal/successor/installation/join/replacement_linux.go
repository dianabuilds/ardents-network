package join

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	"os"
	"path/filepath"
	"slices"
)

// One explicit stopped transition. It retains only independently admitted
// semantic inspectors, never the predecessor's live Node/State/budget owners.
type replacementAttempt struct {
	ctx                context.Context
	verifier           *release.Verifier
	authorization      infrastructure.JoinAuthorization
	input              Inputs
	retired            Started
	candidate          *replacementCandidate
	semantic           *replacementInspections
	identity           *runtimeIdentity
	namespace          *startNamespace
	manager            *systemd.JoinReplacement
	managerIdentity    string
	births             *initialBirths
	mutations          []*fixedfile.Mutation
	pending            *initialFile
	parent             os.FileInfo
	reloaded, complete bool
	terminal           error
}

// Replace consumes original Inputs and verifier. It selects a strictly forward
// stopped generation, without Start, rollback, history reset or prefix recovery.
// Every failure after journal birth retains its blocker and original prefix.
func Replace(v *release.Verifier, a infrastructure.JoinAuthorization, input Inputs, bindingPath, distribution, history string) (returned error) {
	p := input.original
	if p == nil || v == nil || os.Geteuid() != 0 || !validPath(bindingPath) {
		return ErrInput
	}
	if _, ok := p.ctx.Deadline(); !ok {
		return ErrInput
	}
	if err := p.begin(); err != nil {
		return err
	}
	p.mu.Lock()
	if p.installing || p.inspected {
		p.mu.Unlock()
		p.uses.Done()
		return ErrInput
	}
	p.installing = true
	p.mu.Unlock()
	phase := "admission"
	s := &replacementAttempt{ctx: p.ctx, verifier: v, authorization: a, input: input}
	defer func() {
		s.terminal = errors.Join(s.terminal, returned)
		returned = s.close()
		p.uses.Done()
		returned = errors.Join(returned, input.Close(), v.Close(), p.ctx.Err())
		if returned == nil && s.complete {
			returned = publishInitialPending(s.births, s.pending, s.parent)
		}
		if returned != nil && s.births != nil && s.births.journal != nil {
			returned = errors.Join(ErrRepairRequired, returned)
		}
		if returned != nil {
			returned = fmt.Errorf("join replacement %s: %w", phase, returned)
		}
	}()
	if err := installedPlatform(s.ctx); err != nil {
		return err
	}
	if err := errors.Join(input.IndependentPaths(distribution, history), a.Check(s.ctx, v), v.CheckHistoryPath(s.ctx, history)); err != nil {
		return err
	}
	var err error
	s.retired, err = ReadStarted(s.ctx, v, bindingPath, history)
	if err != nil {
		return err
	}
	c := s.retired.original.completed.original
	if !c.hostingPrepared || s.retired.original.retirement == nil {
		return ErrBinding
	}
	s.candidate, err = prepareReplacement(s.retired.original, a, input)
	if err != nil {
		return err
	}
	if err = verifyControllerProgram(s.ctx, a.Program()); err != nil {
		return errors.Join(errControllerProgram, err)
	}
	s.semantic, err = inspectReplacement(s.ctx, c)
	if err != nil {
		return err
	}
	s.identity, err = openRootIdentity(s.ctx)
	if err != nil {
		return err
	}
	s.manager, err = systemd.OpenJoinReplacement(s.ctx)
	if err != nil {
		return err
	}
	s.managerIdentity, err = s.manager.ManagerIdentity()
	if err != nil || s.managerIdentity != s.retired.original.snapshot.archive.Manager {
		return errors.Join(ErrBinding, err)
	}
	facts, err := s.manager.Observe()
	if err != nil {
		return err
	}
	s.namespace, err = openStartNamespace(s.ctx, facts.UnitPaths)
	if err != nil {
		return err
	}
	if err = s.observe(); err != nil {
		return err
	}
	nativeBefore, err := captureNativeContext(s.identity, s.managerIdentity, s.namespace, p)
	if err != nil {
		return err
	}
	b := s.candidate.selection.binding
	root := b.Request.InstallationRoot
	journal := replacementJournal(root, b.Generation)
	beforePath, afterPath := replacementNativePaths(journal)
	sealed := filepath.Join(root, "generations", b.Generation)
	for _, path := range []string{journal, beforePath, afterPath, sealed, replacementPending(root)} {
		if _, err = os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrRepairRequired, err)
		}
	}
	for _, path := range []string{journal, beforePath, afterPath} {
		for _, other := range append(append(b.Request.ownedPaths(), distribution, history), p.path) {
			if contains(path, other) || contains(other, path) {
				return ErrBinding
			}
		}
	}
	allowed := map[string]bool{root: true, b.History: true, sealed: true, replacementPending(root): true, beforePath: true, afterPath: true, filepath.Join(root, "binding.json"): true, "/etc/systemd/system/ardents-join.service": true, filepath.Join(s.retired.original.retirement.path, "retirement-result.json"): true}
	for name := range s.candidate.files {
		allowed[filepath.Join(sealed, name)] = true
	}
	// The pending guard is a sibling of InstallationRoot. Retain its actual
	// original parent, not the installation directory; final publication must
	// compare the same parent inode after every writer/observer has joined.
	pendingParent := c.parents.byPath[filepath.Dir(replacementPending(root))]
	if pendingParent == nil {
		return ErrBinding
	}
	s.parent = pendingParent.info
	if err = s.observe(); err != nil {
		return err
	}
	s.births, err = openJoinBirths(s.ctx, journal, allowed, "join-replacement-v1")
	c.replacing = s
	if err != nil {
		return err
	}
	if err = s.births.record("replacement-admitted", b.History, c.directoryAt(b.History).info, digest(s.candidate.selection.raw), 0700, 0, 0); err != nil {
		return err
	}
	if err = s.births.record("native-context", root, nil, digest(nativeBefore), 0, 0, 0); err != nil {
		return err
	}
	for _, pre := range []struct {
		phase, path, sha string
		mode             os.FileMode
	}{
		{"predecessor-binding", filepath.Join(root, "binding.json"), digest(c.files[bindingPath].body), 0644},
		{"predecessor-unit", "/etc/systemd/system/ardents-join.service", c.binding.UnitSHA256, 0644},
		{"predecessor-retired", filepath.Join(s.retired.original.retirement.path, "retirement-result.json"), s.retired.original.retirement.sha256, 0600},
	} {
		file := c.files[pre.path]
		if pre.phase == "predecessor-retired" {
			file = s.retired.original.retirement.files[pre.path]
		}
		if file == nil {
			return ErrBinding
		}
		if err = s.births.record(pre.phase, pre.path, file.info, pre.sha, pre.mode, 0, 0); err != nil {
			return err
		}
	}
	if err = s.births.write(replacementPending(root), replacementPendingBytes(s.candidate.selection.raw), 0600, 0, 0, s.observe); err != nil {
		return err
	}
	for _, f := range s.births.files {
		if f.path == replacementPending(root) {
			s.pending = f
		}
	}
	if s.pending == nil {
		return ErrBinding
	}
	if err = s.births.write(beforePath, nativeBefore, 0600, 0, 0, s.observe); err != nil {
		return err
	}
	if err = s.births.directory(sealed, 0755, 0, 0, s.observe); err != nil {
		return err
	}
	names := joinGenerationNames()
	slices.Sort(names)
	for _, name := range names {
		mode := os.FileMode(0444)
		if name == programName {
			mode = 0555
		}
		if err = s.births.write(filepath.Join(sealed, name), s.candidate.files[name], mode, 0, 0, s.observe); err != nil {
			return err
		}
	}
	if err = verifyInitialInventory(sealed, s.candidate.files); err != nil {
		return err
	}
	if err = s.births.record("seal-intent", sealed, s.births.identities[sealed], b.Generation, 0555, 0, 0); err != nil {
		return err
	}
	if err = s.observe(); err != nil {
		return err
	}
	s.births.identities[sealed], err = s.births.directoryBirths[sealed].SealRoot()
	if err != nil {
		return err
	}
	if err = s.births.record("sealed", sealed, s.births.identities[sealed], b.Generation, 0555, 0, 0); err != nil {
		return err
	}
	if err = s.replaceFile(bindingPath, s.candidate.selection.raw, "selection-intent", "selection-written"); err != nil {
		return err
	}
	if err = s.replaceFile("/etc/systemd/system/ardents-join.service", s.candidate.unit, "unit-intent", "unit-written"); err != nil {
		return err
	}
	if err = s.births.record("reload-intent", "/etc/systemd/system/ardents-join.service", nil, b.UnitSHA256, 0644, 0, 0); err != nil {
		return err
	}
	if err = s.observe(); err != nil {
		return err
	}
	if err = s.manager.Reload(); err != nil {
		return err
	}
	s.reloaded = true
	if err = s.namespace.reobserveAfterReload(); err != nil {
		return err
	}
	if err = s.observe(); err != nil {
		return err
	}
	nativeAfter, err := captureNativeContext(s.identity, s.managerIdentity, s.namespace, p)
	if err != nil {
		return err
	}
	if err = compareReloadContexts(nativeBefore, nativeAfter); err != nil {
		return err
	}
	if err = s.births.write(afterPath, nativeAfter, 0600, 0, 0, s.observe); err != nil {
		return err
	}
	if err = s.births.record("replacement-verified", bindingPath, nil, digest(s.candidate.selection.raw), 0644, 0, 0); err != nil {
		return err
	}
	hashes := map[string]string{}
	for name, raw := range s.candidate.files {
		hashes[filepath.Join(sealed, name)] = digest(raw)
	}
	old := c.selections[s.candidate.selection.replacement.PredecessorBindingSHA256]
	phase = "journal-readback"
	if _, err = decodeReplacementRecords(s.births.journal.body, s.candidate.selection, old, hashes, nativeBefore, nativeAfter); err != nil {
		return err
	}
	if err = s.observe(); err != nil {
		return err
	}
	phase = "original-close"
	for _, m := range s.mutations {
		if err = m.Close(); err != nil {
			return err
		}
	}
	if err = errors.Join(s.births.close(), s.retired.Close()); err != nil {
		return err
	}
	// Readback has its own original leases. Only the actual still-blocked
	// transition may supply this exact pending inode; ordinary readers refuse it.
	phase = "completed-readback"
	readback, err := readCompletedReplacementPending(s.ctx, bindingPath, s.pending)
	if err != nil {
		return fmt.Errorf("join replacement independent completed readback: %w", err)
	}
	phase = "authorization-readback"
	if err = readback.CheckAuthorization(v, a); err == nil {
		err = s.semantic.check()
	}
	err = errors.Join(err, readback.Close())
	if err != nil {
		return err
	}
	phase = "publication"
	s.complete = true
	return nil
}

func (s *replacementAttempt) observe() error {
	if err := errors.Join(s.ctx.Err(), s.input.Check(), s.authorization.Check(s.ctx, s.verifier), s.verifier.CheckHistoryPath(s.ctx, s.retired.original.completed.original.binding.History)); err != nil {
		return err
	}
	if err := s.retired.Check(); err != nil {
		return err
	}
	if err := errors.Join(s.semantic.check(), s.identity.check(), s.namespace.check()); err != nil {
		return err
	}
	id, err := s.manager.ManagerIdentity()
	if err != nil || id != s.managerIdentity {
		return errors.Join(ErrBinding, err)
	}
	h := s.retired.original
	b := h.completed.original.binding
	if s.reloaded {
		b = s.candidate.selection.binding
	}
	for range 2 {
		if err = cgroup.CheckJoinQuiescent(s.ctx); err != nil {
			return err
		}
		facts, e := s.manager.Observe()
		if e != nil {
			return e
		}
		if err = verifyJoinReplacementStopped(facts.Unit, facts.Service, b, h.snapshot, h.retirement); err != nil {
			return err
		}
		if err = errors.Join(s.retired.Check(), s.semantic.check(), s.identity.check(), s.namespace.check(), s.authorization.Check(s.ctx, s.verifier), s.input.Check()); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}

func (s *replacementAttempt) replaceFile(path string, body []byte, intent, written string) error {
	if err := s.observe(); err != nil {
		return err
	}
	c := s.retired.original.completed.original
	original := c.files[path]
	parent := c.parents.byPath[filepath.Dir(path)]
	if original == nil || parent == nil {
		return ErrBinding
	}
	m, err := fixedfile.Replace(s.ctx, path, parent.info, original.info, original.body, body, 0644, 0)
	if m != nil {
		s.mutations = append(s.mutations, m)
	}
	if err != nil {
		return err
	}
	if err = s.births.record(intent, path, m.Identity(), digest(body), 0644, 0, 0); err != nil {
		return err
	}
	if err = s.observe(); err != nil {
		return err
	}
	if err = original.file.Close(); err != nil {
		return err
	}
	delete(c.files, path)
	c.fileOrder = slices.DeleteFunc(c.fileOrder, func(f *initialFile) bool { return f == original })
	info, err := m.Commit()
	if err != nil {
		return err
	}
	fresh, err := c.read(path, 32<<10, 0644, 0, 0)
	if err != nil || !bytes.Equal(fresh.body, body) || !os.SameFile(info, fresh.info) {
		return errors.Join(ErrBinding, err)
	}
	if err = s.births.record(written, path, info, digest(body), 0644, 0, 0); err != nil {
		return err
	}
	return s.observe()
}
func (s *replacementAttempt) owns(path string) bool {
	if s == nil || s.births == nil {
		return false
	}
	if s.births.journal != nil && s.births.journal.path == path {
		return true
	}
	for _, f := range s.births.files {
		if f.path == path {
			return true
		}
	}
	return false
}
func (s *replacementAttempt) checkBirths() error {
	if s == nil || s.births == nil {
		return ErrBinding
	}
	return s.births.check()
}
func (s *replacementAttempt) extendInventory(inventory map[string]map[string]bool) {
	if s == nil || s.births == nil {
		return
	}
	sealed := filepath.Join(s.candidate.selection.binding.Request.InstallationRoot, "generations", s.candidate.selection.binding.Generation)
	if s.births.dirs[sealed] != nil {
		inventory[filepath.Dir(sealed)][filepath.Base(sealed)] = true
	}
}
func (s *replacementAttempt) close() error {
	for _, m := range s.mutations {
		s.terminal = errors.Join(s.terminal, m.Close())
	}
	s.terminal = errors.Join(s.terminal, s.manager.Close(), s.namespace.close(), s.identity.close(), s.births.close(), s.retired.Close())
	if s.complete && s.terminal == nil {
		s.terminal = errors.Join(s.terminal, s.semantic.check(), s.authorization.Check(s.ctx, s.verifier))
	}
	s.terminal = errors.Join(s.terminal, s.semantic.close())
	return s.terminal
}
