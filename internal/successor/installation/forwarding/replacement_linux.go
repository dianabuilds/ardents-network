package forwarding

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

type replacementAttempt struct {
	ctx                context.Context
	verifier           *release.Verifier
	authorization      infrastructure.ForwardingAuthorization
	input              Inputs
	usingInput         bool
	retired            Started
	candidate          *replacementCandidate
	semantic           *replacementInspections
	identity           *runtimeIdentity
	namespace          *startNamespace
	manager            *systemd.ForwardingReplacement
	managerIdentity    string
	births             *initialBirths
	mutations          []*fixedfile.Mutation
	pending            *initialFile
	parent             os.FileInfo
	reloaded, complete bool
	terminal           error
}

// Replace explicitly selects a strictly forward stopped generation. It consumes
// the original Inputs and verifier, retaining all histories and blockers through
// actual native joins. It never starts a Node, initializes semantic history,
// retries a reload, rolls back a partial effect or recovers an earlier prefix.
func Replace(v *release.Verifier, a infrastructure.ForwardingAuthorization, input Inputs, bindingPath, distribution, history string) (returned error) {
	p := input.original
	if p == nil || v == nil || os.Geteuid() != 0 || !validPath(bindingPath) {
		return ErrInput
	}
	if _, finite := p.ctx.Deadline(); !finite {
		return ErrInput
	}
	s := &replacementAttempt{ctx: p.ctx, verifier: v, authorization: a, input: input}
	if err := p.begin(); err != nil {
		return err
	}
	s.usingInput = true
	p.mu.Lock()
	if p.installing {
		p.mu.Unlock()
		p.uses.Done()
		return ErrInput
	}
	p.installing = true
	p.mu.Unlock()
	defer func() {
		s.terminal = errors.Join(s.terminal, returned)
		returned = s.close()
		returned = errors.Join(returned, finishTransition(s.ctx))
		if returned == nil && s.complete {
			returned = publishInitialPending(s.births, s.pending, s.parent)
		}
		if returned != nil && s.births != nil && s.births.journal != nil {
			returned = errors.Join(ErrRepairRequired, returned)
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
	if err := c.readHostingPreparation(bindingPath); err != nil {
		return err
	}
	s.candidate, err = prepareReplacement(s.retired.original, a, input)
	if err != nil {
		return err
	}
	if err := verifyControllerProgram(s.ctx, a.Program()); err != nil {
		return err
	}
	s.semantic, err = inspectReplacement(s.ctx, c)
	if err != nil {
		return err
	}
	s.identity, err = openRootIdentity(s.ctx)
	if err != nil {
		return err
	}
	s.manager, err = systemd.OpenForwardingReplacement(s.ctx)
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
	if err := s.observe(); err != nil {
		return err
	}
	context, err := replacementContext(s.identity, s.managerIdentity, s.namespace, p, c.files)
	if err != nil {
		return err
	}
	b := s.candidate.selection.binding
	root := b.Request.InstallationRoot
	sealed := filepath.Join(root, "generations", b.Generation)
	journal := replacementJournal(root, b.Generation)
	beforePath, afterPath := replacementNativePaths(journal)
	for _, resource := range []string{journal, beforePath, afterPath} {
		for _, path := range append(append(b.Request.ownedPaths(), distribution, history), p.path) {
			if contains(path, resource) || contains(resource, path) {
				return ErrBinding
			}
		}
		if _, err := os.Lstat(resource); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrRepairRequired, err)
		}
	}
	allowed := map[string]bool{sealed: true, replacementPending(root): true, beforePath: true, afterPath: true, root: true, b.History: true,
		filepath.Join(root, "binding.json"): true, "/etc/systemd/system/ardents-forwarding.service": true,
		filepath.Join(s.retired.original.latestRetirement.path, "retirement-result.json"): true}
	for name := range s.candidate.files {
		allowed[filepath.Join(sealed, name)] = true
	}
	s.parent = c.files.dirs[root].info
	// Context construction performs real reads. Reobserve accepting owners
	// after that I/O and immediately before the first exclusive native birth.
	if err := s.observe(); err != nil {
		return err
	}
	if err := bindTransition(s.ctx, "replacement", journal, b.Generation, s.candidate.selection.raw, context, s.managerIdentity); err != nil {
		return err
	}
	s.births, err = openForwardingBirths(s.ctx, journal, allowed, "forwarding-replacement-v2")
	if err != nil {
		return err
	}
	if err := s.births.record("replacement-admitted", b.History, c.files.dirs[b.History].info, digest(s.candidate.selection.raw), 0700, 0, 0); err != nil {
		return err
	}
	if err := s.births.record("native-context", root, nil, digest(context), 0, 0, 0); err != nil {
		return err
	}
	for _, preimage := range []struct{ phase, path, sha string }{
		{"predecessor-binding", filepath.Join(root, "binding.json"), digest(c.files.files[filepath.Join(root, "binding.json")].body)},
		{"predecessor-unit", "/etc/systemd/system/ardents-forwarding.service", c.binding.UnitSHA256},
		{"predecessor-retired", filepath.Join(s.retired.original.latestRetirement.path, "retirement-result.json"), s.retired.original.latestRetirement.sha256},
	} {
		var info os.FileInfo
		mode := os.FileMode(0644)
		if preimage.phase == "predecessor-retired" {
			info = s.retired.original.latestRetirement.files[preimage.path].info
			mode = 0600
		} else {
			info = c.files.files[preimage.path].info
		}
		if err := s.births.record(preimage.phase, preimage.path, info, preimage.sha, mode, 0, 0); err != nil {
			return err
		}
	}
	if err := s.births.write(replacementPending(root), replacementPendingBytes(s.candidate.selection.raw), 0600, 0, 0, s.observe); err != nil {
		return err
	}
	for _, file := range s.births.files {
		if file.path == replacementPending(root) {
			s.pending = file
		}
	}
	if s.pending == nil {
		return ErrBinding
	}
	if err := s.births.write(beforePath, context, 0600, 0, 0, s.observe); err != nil {
		return err
	}
	if err := s.births.directory(sealed, 0755, 0, 0, s.observe); err != nil {
		return err
	}
	names := forwardingGenerationNames()
	slices.Sort(names)
	for _, name := range names {
		mode := os.FileMode(0444)
		if name == programName {
			mode = 0555
		}
		if err := s.births.write(filepath.Join(sealed, name), s.candidate.files[name], mode, 0, 0, s.observe); err != nil {
			return err
		}
	}
	if err := verifyInitialInventory(sealed, s.candidate.files); err != nil {
		return err
	}
	if err := s.births.record("seal-intent", sealed, s.births.identities[sealed], b.Generation, 0555, 0, 0); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.births.dirs[sealed].Chmod(0555); err != nil {
		return err
	}
	if err := s.births.dirs[sealed].Sync(); err != nil {
		return err
	}
	s.births.identities[sealed], err = s.births.dirs[sealed].Stat()
	if err != nil || !nodeAccess(s.births.identities[sealed], 0555, 0, 0, true) {
		return errors.Join(ErrBinding, err)
	}
	if err := s.births.record("sealed", sealed, s.births.identities[sealed], b.Generation, 0555, 0, 0); err != nil {
		return err
	}
	if err := s.replaceFile(filepath.Join(root, "binding.json"), s.candidate.selection.raw, "selection-intent", "selection-written"); err != nil {
		return err
	}
	if err := s.replaceFile("/etc/systemd/system/ardents-forwarding.service", s.candidate.unit, "unit-intent", "unit-written"); err != nil {
		return err
	}
	if err := s.births.record("reload-intent", "/etc/systemd/system/ardents-forwarding.service", nil, b.UnitSHA256, 0644, 0, 0); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.manager.Reload(); err != nil {
		return err
	}
	s.reloaded = true
	if err := s.namespace.reobserveAfterReload(); err != nil {
		return err
	}
	if err := c.files.observeOwnedReload(s.namespace); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	currentContext, err := nativeContextAfterReload(context, s.namespace)
	if err != nil {
		return err
	}
	if err := observeTransitionReload(s.ctx, s.namespace); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.births.write(afterPath, currentContext, 0600, 0, 0, s.observe); err != nil {
		return err
	}
	if err := s.births.record("replacement-verified", filepath.Join(root, "binding.json"), nil, digest(s.candidate.selection.raw), 0644, 0, 0); err != nil {
		return err
	}
	hashes := map[string]string{}
	for name, raw := range s.candidate.files {
		hashes[name] = digest(raw)
	}
	previous := c.selections[s.candidate.selection.replacement.PredecessorBindingSHA256]
	if _, err := decodeReplacementContextRecords(s.births.journal.body, s.candidate.selection, previous, hashes, context, currentContext); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	// Ordinary readers still refuse the pending marker. An independent readback
	// is acquired only after the original history/writer leases actually close.
	for _, mutation := range s.mutations {
		if err := mutation.Close(); err != nil {
			return err
		}
	}
	if err := errors.Join(s.births.close(), s.retired.Close()); err != nil {
		return err
	}
	readback, err := readCompletedPending(s.ctx, bindingPath, s.pending)
	if err != nil {
		return err
	}
	checked := &started{invocationChain: readback.original.files.invocations, ctx: s.ctx, verifier: v, completed: readback}
	if err == nil {
		err = (Started{original: checked}).Check()
	}
	if err == nil {
		err = readback.CheckAuthorization(v, a)
	}
	err = errors.Join(err, checked.close())
	if err != nil {
		return err
	}
	s.complete = true
	return nil
}

func (s *replacementAttempt) observe() error {
	if err := errors.Join(s.ctx.Err(), s.authorization.Check(s.ctx, s.verifier), s.input.Check()); err != nil {
		return err
	}
	if err := s.checkFiles(); err != nil {
		return err
	}
	if err := s.retired.original.checkChain(nil, nil); err != nil {
		return err
	}
	if err := s.semantic.check(); err != nil {
		return err
	}
	if err := errors.Join(s.identity.check(), s.namespace.check()); err != nil {
		return err
	}
	identity, err := s.manager.ManagerIdentity()
	if err != nil || identity != s.managerIdentity {
		return errors.Join(ErrBinding, err)
	}
	h := s.retired.original
	for round := 0; round < 2; round++ {
		if err := cgroup.CheckForwardingQuiescent(s.ctx); err != nil {
			return err
		}
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		if s.reloaded {
			err = verifyForwardingReplacementStopped(facts.Unit, facts.Service, s.candidate.selection, h.snapshot.archive, h.snapshot.number, h.snapshot.id, h.latestRetirement.result, h.latestRetirement.sha256)
		} else {
			err = verifyForwardingRestartStopped(facts.Unit, facts.Service, h.completed.original.binding.Request, h.completed.original.binding.Generation, h.snapshot.archive, h.snapshot.id, h.latestRetirement.result.execution())
		}
		if err != nil {
			return err
		}
		if err := errors.Join(s.identity.check(), s.namespace.check(), s.checkFiles(), s.semantic.check(), s.authorization.Check(s.ctx, s.verifier), s.input.Check()); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}

func (s *replacementAttempt) replaceFile(path string, body []byte, intent, written string) error {
	if err := s.observe(); err != nil {
		return err
	}
	f := s.retired.original.completed.original.files
	original := f.files[path]
	if original == nil || f.dirs[filepath.Dir(path)] == nil {
		return ErrBinding
	}
	mutation, err := fixedfile.Replace(s.ctx, path, f.dirs[filepath.Dir(path)].info, original.info, original.body, body, 0644, 0)
	if mutation != nil {
		s.mutations = append(s.mutations, mutation)
	}
	if err != nil {
		return err
	}
	if err := s.births.record(intent, path, mutation.Identity(), digest(body), 0644, 0, 0); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := original.file.Close(); err != nil {
		return err
	}
	delete(f.files, path)
	f.fileOrder = slices.DeleteFunc(f.fileOrder, func(file *initialFile) bool { return file == original })
	info, err := mutation.Commit()
	if err != nil {
		return err
	}
	raw, err := f.read(path, 32<<10, 0644, 0, 0)
	if err != nil || !bytes.Equal(raw, body) || !os.SameFile(info, f.files[path].info) {
		return errors.Join(ErrBinding, err)
	}
	if err := s.births.record(written, path, info, digest(body), 0644, 0, 0); err != nil {
		return err
	}
	return s.observe()
}

// This operation-local view permits only actual original births. Ordinary
// Completed observers never receive this view, a bypass flag or a new right.
func (s *replacementAttempt) checkFiles() error {
	f := s.retired.original.completed.original.files
	if err := f.local(); err != nil {
		return err
	}
	view := &completedFiles{ctx: s.ctx, files: f.files, fileOrder: f.fileOrder, dirs: f.dirs, dirOrder: f.dirOrder, clock: f.clock, pending: f.pending,
		sealed: f.sealed, credential: f.credential, absentPaths: slices.Clone(f.absentPaths), extraInventories: f.extraInventories, generationRoot: f.generationRoot, generationNames: slices.Clone(f.generationNames), replacementJournals: slices.Clone(f.replacementJournals), recovered: f.recovered, recoveryContext: f.recoveryContext, recoveredReplacements: f.recoveredReplacements, invocations: f.invocations}
	if s.births != nil {
		if err := s.births.check(); err != nil {
			return err
		}
		root := s.candidate.selection.binding.Request.InstallationRoot
		beforePath, afterPath := replacementNativePaths(s.births.journal.path)
		view.replacementJournals = append(view.replacementJournals, s.births.journal.path)
		for _, file := range s.births.files {
			if file.path == replacementPending(root) {
				view.absentPaths = slices.DeleteFunc(view.absentPaths, func(path string) bool { return path == file.path })
			}
			if file.path == beforePath || file.path == afterPath {
				view.replacementJournals = append(view.replacementJournals, file.path)
			}
		}
		sealed := filepath.Join(root, "generations", s.candidate.selection.binding.Generation)
		if s.births.dirs[sealed] != nil {
			view.generationNames = append(view.generationNames, s.candidate.selection.binding.Generation)
			slices.Sort(view.generationNames)
		}
	}
	return errors.Join(view.check(), f.local())
}

func (s *replacementAttempt) close() error {
	for _, mutation := range s.mutations {
		s.terminal = errors.Join(s.terminal, mutation.Close())
	}
	s.terminal = errors.Join(s.terminal, s.manager.Close(), s.namespace.close(), s.identity.close(), s.births.close(), s.retired.Close())
	if s.complete && s.terminal == nil {
		s.terminal = errors.Join(s.terminal, s.semantic.check(), s.authorization.Check(s.ctx, s.verifier))
	}
	s.terminal = errors.Join(s.terminal, s.semantic.close())
	if s.usingInput {
		s.usingInput = false
		s.input.original.uses.Done()
		s.terminal = errors.Join(s.terminal, s.input.Close())
	}
	s.terminal = errors.Join(s.terminal, s.verifier.Close(), s.ctx.Err())
	return s.terminal
}
