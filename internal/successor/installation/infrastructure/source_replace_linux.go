package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

type sourceReplacement struct {
	ctx                context.Context
	verifier           *release.Verifier
	authorization      Authorization
	input              SourceInputs
	semantic           state.SourceInspection
	usingInput         bool
	retired            *sourceRetirement
	candidate          *sourceReplacementCandidate
	identity           *sourceRuntimeIdentity
	managerIdentity    string
	manager            *systemd.SourceReplacement
	namespace          *sourceNamespace
	births             *sourceBirths
	mutations          []*fixedfile.Mutation
	readback           *sourceInitial
	pending            *sourceBirth
	rootInfo           os.FileInfo
	reloaded, verified bool
	terminal           error
}

// ReplaceSource consumes one original explicit retirement and a strictly forward
// coherent Release pair. It preserves the account, credentials, semantic roots,
// all predecessor bytes and original history. Selection and manager reload are
// separate effects; unknown/partial completion retains the exact blocker and
// journal. It performs no Start, repair, rollback, adoption or crash continuation.
// This finite operation consumes/joins input custody before its final commit.
func ReplaceSource(ctx context.Context, v *release.Verifier, a Authorization, input SourceInputs, semantic state.SourceInspection, binding, distribution, history string) (err error) {
	if ctx == nil || v == nil || input.original == nil || os.Geteuid() != 0 {
		return ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return ErrInput
	}
	s := &sourceReplacement{ctx: ctx, verifier: v, authorization: a, input: input, semantic: semantic}
	defer func() {
		s.terminal = errors.Join(s.terminal, err)
		err = s.close()
		if err == nil && s.verified {
			err = s.publish()
		}
		if err != nil && s.births != nil && s.births.journal != nil {
			err = errors.Join(ErrSourceRepairRequired, err)
		}
	}()
	if err := input.original.begin(); err != nil {
		return err
	}
	s.usingInput = true
	if err := sourceInstalledPlatform(ctx); err != nil {
		return err
	}
	if err := errors.Join(input.IndependentPaths(distribution, history), a.Check(ctx, v)); err != nil {
		return err
	}
	s.retired, err = openSourceRetirement(ctx, v, binding, history)
	if err != nil {
		return err
	}
	s.candidate, err = prepareSourceReplacement(s.retired, a, input)
	if err != nil {
		return err
	}
	if err := verifySourceControllerProgram(ctx, a.Program()); err != nil {
		return err
	}
	s.identity, err = openSourceRootIdentity(ctx)
	if err != nil {
		return err
	}
	s.managerIdentity, err = sourceManagerIdentity()
	if err != nil {
		return err
	}
	s.manager, err = systemd.OpenSourceReplacement(ctx)
	if err != nil {
		return err
	}
	facts, err := s.manager.Observe()
	if err != nil {
		return err
	}
	s.namespace, err = openSourceNamespace(ctx, facts.UnitPaths)
	if err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	old := s.retired.chain.initial.selection
	b := s.candidate.binding.Configuration
	root, sealed := b.Request.InstallationRoot, filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	s.rootInfo = old.identities[root]
	journal := sourceReplacementJournal(root, b.Generation)
	allowed := map[string]bool{sealed: true, filepath.Join(root, "source-replacement-pending"): true}
	for name := range s.candidate.files {
		allowed[filepath.Join(sealed, name)] = true
	}
	for _, path := range append(b.Request.paths(), distribution, history, input.original.requestPath) {
		if contains(path, journal) || contains(journal, path) {
			return ErrBinding
		}
	}
	s.births, err = openSourceBirths(ctx, journal, allowed)
	if err != nil {
		return err
	}
	s.retired.chain.initial.transition = s.births
	admitted, err := s.retired.chain.initial.history.selectedCommitment(v, s.candidate.raw)
	if err != nil {
		return err
	}
	if err := s.births.record("replacement-admitted", root, nil, admitted, 0, b.UID, b.GID); err != nil {
		return err
	}
	selected, unit := filepath.Join(root, "binding.json"), "/etc/systemd/system/ardents-source.service"
	for _, record := range []struct {
		phase, path, sha string
		info             os.FileInfo
		mode             os.FileMode
	}{
		{"predecessor-binding", selected, s.candidate.binding.PredecessorBindingSHA256, old.files[selected].info, 0644},
		{"predecessor-unit", unit, old.binding.UnitSHA256, old.files[unit].info, 0644},
		{"predecessor-retired", filepath.Join(s.retired.chain.latest.number.directory(root), "source-retirement.json"), s.retired.sha256,
			old.files[filepath.Join(s.retired.chain.latest.number.directory(root), "source-retirement.json")].info, 0600},
	} {
		if err := s.births.record(record.phase, record.path, record.info, record.sha, record.mode, 0, 0); err != nil {
			return err
		}
	}
	pending := filepath.Join(root, "source-replacement-pending")
	if err := s.births.write(pending, sourceReplacementPending(digest(s.candidate.raw)), 0600, 0, 0, s.observe); err != nil {
		return err
	}
	for _, file := range s.births.files {
		if file.path == pending {
			s.pending = file
		}
	}
	if s.pending == nil {
		return ErrBinding
	}
	if err := s.births.directory(sealed, 0755, 0, 0, s.observe); err != nil {
		return err
	}
	names := sourceGenerationNames()
	slices.Sort(names)
	for _, name := range names {
		mode := os.FileMode(0444)
		if name == ProgramName {
			mode = 0555
		}
		if err := s.births.write(filepath.Join(sealed, name), s.candidate.files[name], mode, 0, 0, s.observe); err != nil {
			return err
		}
	}
	if err := checkSourceInventory(sealed, names); err != nil {
		return err
	}
	if err := s.births.record("seal-intent", sealed, s.births.directories[sealed], b.Generation, 0555, 0, 0); err != nil {
		return err
	}
	if err := sealSourceDirectory(s.births, sealed, s.observe); err != nil {
		return err
	}
	if err := s.births.record("sealed", sealed, s.births.directories[sealed], b.Generation, 0555, 0, 0); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.replaceFile(selected, s.candidate.raw, "selection-intent", "selection-written"); err != nil {
		return err
	}
	if err := s.replaceFile(unit, s.candidate.unit, "unit-intent", "unit-written"); err != nil {
		return err
	}
	if err := s.births.record("reload-intent", unit, nil, digest(s.candidate.unit), 0644, 0, 0); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.manager.Reload(); err != nil {
		return err
	}
	if err := s.namespace.reobserveAfterReload(); err != nil {
		return err
	}
	s.reloaded = true
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.births.record("replacement-verified", selected, nil, digest(s.candidate.raw), 0644, 0, 0); err != nil {
		return err
	}
	s.readback, err = openSourceInitialTransition(ctx, v, selected, history, s.births)
	if err != nil {
		return err
	}
	if err := s.readback.check(); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	s.verified = true
	return nil
}

// The two old read observers join before their physical mutation owners take
// responsibility. Only exact successful Commit obtains a new independent read
// observer. Other predecessor resources and all original history leases stay
// retained; no old Authorization or byte observer is revived for new work.
func (s *sourceReplacement) replaceFile(path string, body []byte, intent, written string) error {
	if err := s.observe(); err != nil {
		return err
	}
	old := s.retired.chain.initial.selection
	original := old.files[path]
	if original == nil {
		return ErrBinding
	}
	mutation, err := fixedfile.Replace(s.ctx, path, old.identities[filepath.Dir(path)], original.info, original.raw, body, 0644, 0)
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
	delete(old.files, path)
	info, err := mutation.Commit()
	if err != nil {
		return err
	}
	raw, err := old.read(path, 32<<10, 0644, 0, 0)
	if err != nil || !bytes.Equal(raw, body) || old.files[path] == nil || !os.SameFile(info, old.files[path].info) {
		return errors.Join(ErrBinding, err)
	}
	if err := s.births.record(written, path, info, digest(body), 0644, 0, 0); err != nil {
		return err
	}
	return s.observe()
}

func (s *sourceReplacement) observe() error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := s.authorization.Check(s.ctx, s.verifier); err != nil {
		return err
	}
	if err := s.input.Check(); err != nil {
		return err
	}
	if err := s.retired.check(); err != nil {
		return err
	}
	b := s.candidate.binding.Configuration
	if err := s.checkSemantic(b.UID, b.GID); err != nil {
		return err
	}
	if err := observeSourcePrincipal(s.ctx, b.UID, b.GID); err != nil {
		return err
	}
	if err := s.identity.check(); err != nil {
		return err
	}
	identity, err := sourceManagerIdentity()
	if err != nil || identity != s.managerIdentity {
		return errors.Join(ErrBinding, err)
	}
	if err := s.namespace.check(); err != nil {
		return err
	}
	for round := 0; round < 2; round++ {
		if err := cgroup.CheckSourceQuiescent(s.ctx); err != nil {
			return err
		}
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		if s.reloaded {
			err = s.retired.stoppedFor(facts, b.Request, b.Generation)
		} else {
			err = s.retired.stopped(facts)
		}
		if err != nil {
			return err
		}
		if err := s.authorization.Check(s.ctx, s.verifier); err != nil {
			return err
		}
		if err := s.retired.check(); err != nil {
			return err
		}
		if err := s.input.Check(); err != nil {
			return err
		}
		if err := s.namespace.check(); err != nil {
			return err
		}
		if err := cgroup.CheckSourceQuiescent(s.ctx); err != nil {
			return err
		}
	}
	return errors.Join(s.checkSemantic(b.UID, b.GID), s.ctx.Err())
}

func (s *sourceReplacement) checkSemantic(uid, gid uint32) error {
	raw, err := s.input.ServingPlan()
	if err != nil {
		return err
	}
	return s.semantic.Check(s.ctx, raw, uid, gid)
}

func (s *sourceReplacement) close() error {
	if s.readback != nil {
		s.terminal = errors.Join(s.terminal, s.readback.close())
	}
	if s.namespace != nil {
		s.terminal = errors.Join(s.terminal, s.namespace.close())
	}
	if s.identity != nil {
		s.terminal = errors.Join(s.terminal, s.identity.close())
	}
	if s.manager != nil {
		s.terminal = errors.Join(s.terminal, s.manager.Close())
	}
	for _, mutation := range s.mutations {
		s.terminal = errors.Join(s.terminal, mutation.Close())
	}
	if s.retired != nil {
		s.terminal = errors.Join(s.terminal, s.retired.close())
	}
	s.terminal = errors.Join(s.terminal, s.births.close())
	if s.verified && s.terminal == nil {
		b := s.candidate.binding.Configuration
		s.terminal = errors.Join(s.terminal, s.checkSemantic(b.UID, b.GID))
	}
	s.terminal = errors.Join(s.terminal, s.semantic.Close())
	if s.usingInput {
		s.usingInput = false
		s.input.original.uses.Done()
		s.terminal = errors.Join(s.terminal, s.input.Close())
	}
	return errors.Join(s.terminal, s.ctx.Err())
}

// Every original observer, native writer and input has joined before the final
// original blocker unlink. A lost unlink restores refusal after a crash; no
// required sync or manager effect follows it. Release remains open in the
// original command and is rechecked independently at this final commit point.
func (s *sourceReplacement) publish() error {
	if !s.verified || s.terminal != nil || s.pending == nil || s.rootInfo == nil || s.births == nil || !s.births.closed || s.births.terminal != nil {
		return ErrBinding
	}
	if err := s.authorization.Check(s.ctx, s.verifier); err != nil {
		return err
	}
	root := s.candidate.binding.Configuration.Request.InstallationRoot
	info, err := os.Lstat(root)
	if err != nil || !os.SameFile(s.rootInfo, info) || !sourceAccess(info, 0755, 0, 0, true) {
		return errors.Join(ErrBinding, err)
	}
	pending, err := os.Lstat(s.pending.path)
	if err != nil || !sameSourceFile(s.pending.info, pending) {
		return errors.Join(ErrBinding, err)
	}
	return os.Remove(s.pending.path)
}
