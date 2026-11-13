package join

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

type recoveryPublication struct {
	births  *initialBirths
	pending *initialFile
}
type recoveryReplacement struct {
	ctx           context.Context
	verifier      *release.Verifier
	authorization infrastructure.JoinAuthorization
	input         Inputs
	completed     Completed
	native        *recoveryContext
	manager       *systemd.JoinRecoveryReplacement
	semantic      *replacementInspections
	publication   recoveryPublication
	terminal      error
}

// RecoverReplacement admits only the proven complete62-record stopped cut.
// Earlier, uncertain or unjoined prefixes retain their original blockers. It
// performs neither Reload nor Start, and never rewrites an original journal.
func RecoverReplacement(v *release.Verifier, a infrastructure.JoinAuthorization, input Inputs, bindingPath, distribution, history string) (returned error) {
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
	released := false
	s := &recoveryReplacement{ctx: p.ctx, verifier: v, authorization: a, input: input}
	defer func() {
		s.terminal = errors.Join(s.terminal, returned)
		returned = s.close()
		if !released {
			p.uses.Done()
		}
		returned = errors.Join(returned, input.Close(), v.Close(), p.ctx.Err())
		if returned != nil {
			returned = errors.Join(ErrRepairRequired, returned)
		}
	}()
	if err := installedPlatform(s.ctx); err != nil {
		return err
	}
	if err := errors.Join(input.IndependentPaths(distribution, history), a.Check(s.ctx, v), v.CheckHistoryPath(s.ctx, history), verifyControllerProgram(s.ctx, a.Program())); err != nil {
		return err
	}
	root := p.request.InstallationRoot
	if bindingPath != filepath.Join(root, "binding.json") {
		return ErrBinding
	}
	raw, info, err := readInitialOriginal(s.ctx, replacementPending(root), 4096, 0600, 0, 0, false)
	if err != nil {
		return err
	}
	originalPending := &initialFile{path: replacementPending(root), info: info, body: raw}
	s.completed, err = readCompletedReplacementPending(s.ctx, bindingPath, originalPending)
	if err != nil {
		return err
	}
	c := s.completed.original
	if err = errors.Join(s.completed.IndependentPaths(distribution, history), s.completed.CheckAuthorization(v, a)); err != nil {
		return err
	}
	if c.recoveryCut == nil || c.recoveryOriginal == nil || !c.hostingPrepared {
		return ErrBinding
	}
	selection := c.selections[digest(c.files[bindingPath].body)]
	if selection.replacement == nil || c.recoveryCut.Boundary.Journal != replacementJournal(root, c.binding.Generation) {
		return ErrBinding
	}
	delivery, err := p.readDelivery()
	if err != nil {
		return err
	}
	defer func() {
		for _, f := range delivery {
			clear(f.raw)
		}
	}()
	for n, name := range []string{"join-request.json", "join-plan.json"} {
		if !bytes.Equal(delivery[n].raw, c.files[filepath.Join(c.sealed, name)].body) {
			return ErrBinding
		}
	}
	for n, name := range []string{"certificate.pem", "private-key.pem"} {
		if !bytes.Equal(delivery[n+2].raw, c.files[filepath.Join(c.binding.Request.CredentialDirectory, name)].body) {
			return ErrBinding
		}
	}
	s.semantic, err = inspectStoppedHistories(s.ctx, c)
	if err != nil {
		return err
	}
	s.native, err = openRecoveryContext(s.ctx, c.recoveryCut.Boundary.NativeContext)
	if err != nil {
		return err
	}
	s.manager, err = systemd.OpenJoinRecoveryReplacement(s.ctx, c.recoveryCut.Boundary.Manager)
	if err != nil {
		return err
	}
	if err = s.observe(); err != nil {
		return err
	}
	j := c.files[c.recoveryCut.Boundary.Journal]
	completion := recoveryCompletion{"ardents-join-recovery-completion-v1", j.path, c.recoveryCut.Boundary.Device, c.recoveryCut.Boundary.Inode, digest(j.body), digest(c.recoveryOriginal.body), digest(selection.raw), "selected-stopped"}
	body, err := json.Marshal(completion)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if _, err = decodeRecoveryCompletion(body, j.body, c.recoveryOriginal.body, *c.recoveryCut); err != nil {
		return err
	}
	journal, result, pending := recoveryPaths(j.path)
	for _, path := range []string{journal, result, pending} {
		if _, err = os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	parent := c.recoveryParent(originalPending.path)
	if parent == nil {
		return ErrBinding
	}
	s.publication.births, err = openJoinBirths(s.ctx, journal, map[string]bool{result: true, pending: true}, "join-recovery-v1")
	c.recoveryPublishing = &s.publication
	if err != nil {
		return err
	}
	births := s.publication.births
	if err = births.write(pending, recoveryPendingBytes(body), 0600, 0, 0, s.observe); err != nil {
		return err
	}
	s.publication.pending = births.files[len(births.files)-1]
	if err = births.record("recovery-admitted", result, nil, digest(body), 0600, 0, 0); err != nil {
		return err
	}
	if err = births.write(result, body, 0600, 0, 0, s.observe); err != nil {
		return err
	}
	resultFile := births.files[len(births.files)-1]
	if err = births.record("recovery-verified", result, resultFile.info, digest(body), 0600, 0, 0); err != nil {
		return err
	}
	if err = errors.Join(decodeRecoveryRecords(births.journal, resultFile), s.observe()); err != nil {
		return err
	}
	// All original recovery writers and independent inspectors close before a
	// separate reader reacquires the original/cut/suffix exclusions.
	if err = errors.Join(births.close(), s.manager.Close(), s.native.close(), s.semantic.close(), s.completed.Close()); err != nil {
		return err
	}
	readback, err := readCompletedRecoveryPending(s.ctx, bindingPath, originalPending, &s.publication)
	if err != nil {
		return err
	}
	if err = readback.CheckAuthorization(v, a); err == nil {
		err = readback.CheckHistoryPath(history)
	}
	check := &recoveryReplacement{ctx: s.ctx, verifier: v, authorization: a, input: input, completed: readback}
	if err == nil {
		check.semantic, err = inspectStoppedHistories(s.ctx, readback.original)
	}
	if err == nil {
		check.native, err = openRecoveryContext(s.ctx, c.recoveryCut.Boundary.NativeContext)
	}
	if err == nil {
		check.manager, err = systemd.OpenJoinRecoveryReplacement(s.ctx, c.recoveryCut.Boundary.Manager)
	}
	if err == nil {
		err = check.observe()
	}
	err = errors.Join(err, check.close(), input.Check(), a.Check(s.ctx, v))
	if err != nil {
		return err
	}
	// Publication happens only after the outer defer has joined original input
	// and verifier too; the two guarded unlinks preserve uncertainty on failure.
	released = true
	p.uses.Done()
	if err = errors.Join(input.Close(), v.Close(), s.ctx.Err()); err != nil {
		return err
	}
	if err = publishInitialPending(births, originalPending, parent); err != nil {
		return err
	}
	if err = syncInitialParent(originalPending.path); err != nil {
		return err
	}
	if err = publishInitialPending(births, s.publication.pending, parent); err != nil {
		return err
	}
	return syncInitialParent(s.publication.pending.path)
}
func (s *recoveryReplacement) observe() error {
	if err := errors.Join(s.ctx.Err(), s.input.Check(), s.authorization.Check(s.ctx, s.verifier), s.completed.Check(), s.semantic.check(), s.native.check()); err != nil {
		return err
	}
	c := s.completed.original
	h := c.invocations
	if h == nil || len(h.starts) == 0 || len(h.starts) != len(h.retirements) {
		return ErrBinding
	}
	latest := len(h.starts) - 1
	selection := c.selections[digest(c.files[filepath.Join(c.binding.Request.InstallationRoot, "binding.json")].body)]
	if selection.replacement == nil || uint32(latest) != selection.replacement.PredecessorAttempt || h.retirements[latest].sha256 != selection.replacement.RetirementSHA256 {
		return ErrBinding
	}
	id, err := s.manager.ManagerIdentity()
	if err != nil || id != s.native.expected.Manager {
		return errors.Join(ErrBinding, err)
	}
	for range 2 {
		if err = cgroup.CheckJoinQuiescent(s.ctx); err != nil {
			return err
		}
		facts, e := s.manager.Observe()
		if e != nil {
			return e
		}
		if err = verifyJoinReplacementStopped(facts.Unit, facts.Service, c.binding, h.starts[latest], h.retirements[latest]); err != nil {
			return err
		}
		if err = errors.Join(s.completed.Check(), s.semantic.check(), s.native.check(), s.input.Check(), s.authorization.Check(s.ctx, s.verifier)); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}
func (s *recoveryReplacement) close() error {
	if s == nil {
		return nil
	}
	s.terminal = errors.Join(s.terminal, s.publication.births.close(), s.manager.Close(), s.native.close(), s.semantic.close(), s.completed.Close())
	return s.terminal
}
