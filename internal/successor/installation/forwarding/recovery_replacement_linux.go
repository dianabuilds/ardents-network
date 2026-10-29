package forwarding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

type recoveryReplacementAttempt struct {
	ctx          context.Context
	v            *release.Verifier
	a            infrastructure.ForwardingAuthorization
	input        Inputs
	original     *recoveryOriginal
	candidate    *recoveryCandidate
	history      *started
	semantic     *replacementInspections
	native       *recoveryContext
	manager      *systemd.ForwardingReplacement
	publication  *recoveryPublication
	terminal     error
	beforeReload *recoveredReplacement
	proof        recoveryReloadProof
	after        []byte
}

func stoppedReplacementCompletion(original *recoveryOriginal, candidate *recoveryCandidate, previous generationSelection, hashes map[string]string, before, after []byte) ([]byte, error) {
	logical, err := stoppedReplacementTranscript(original.prefix.journal.body, original.cut, candidate.selection, previous, hashes, before, after)
	if err != nil {
		return nil, err
	}
	boundary := original.cut.Boundary
	delta := []json.RawMessage{}
	if boundary.Records == 61 {
		lines := bytes.Split(logical[:len(logical)-1], []byte{'\n'})
		delta = append(delta, bytes.Clone(lines[61]))
	}
	completion := recoveryCompletion{Schema: "ardents-forwarding-recovery-completion-v1", Operation: "replacement", Journal: boundary.Journal, Device: boundary.Device, Inode: boundary.Inode,
		PrefixSHA256: digest(original.prefix.journal.body), PrefixRecords: boundary.Records, Generation: candidate.selection.binding.Generation, BindingSHA256: digest(candidate.selection.raw),
		OriginalCutSHA256: digest(original.result.body), Outcome: "selected-stopped", Records: delta, TranscriptSHA256: digest(logical)}
	raw, err := json.Marshal(completion)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if _, _, err := decodeRecoveryCompletion(raw, original.prefix.journal.body, original.result.body, original.cut); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *recoveryReplacementAttempt) run(bindingPath, historyPath string) error {
	completed, err := readCompletedInterrupted(s.ctx, bindingPath, s.original)
	if err != nil {
		return err
	}
	s.history = &started{invocationChain: completed.original.files.invocations, ctx: s.ctx, verifier: s.v, completed: completed}
	c := completed.original
	if err := errors.Join(completed.CheckHistoryPath(historyPath), completed.CheckAuthorization(s.v, s.a), s.v.CheckHistoryPath(s.ctx, historyPath)); err != nil {
		return err
	}
	selected := s.candidate.selection
	r := selected.replacement
	if r == nil || s.history.snapshot == nil || s.history.latestRetirement == nil || uint32(s.history.snapshot.number) != r.PredecessorAttempt || s.history.latestRetirement.sha256 != r.RetirementSHA256 || s.history.latestRetirement.bindingSHA != r.PredecessorBindingSHA256 || s.history.snapshot.archive.Generation != r.PredecessorGeneration {
		return ErrBinding
	}
	if err := c.readHostingPreparation(bindingPath); err != nil {
		return err
	}
	sealed := filepath.Join(c.binding.Request.InstallationRoot, "generations", c.binding.Generation)
	hashes := map[string]string{}
	for name, raw := range s.candidate.files {
		file := c.files.files[filepath.Join(sealed, name)]
		if file == nil || !bytes.Equal(file.body, raw) {
			return ErrBinding
		}
		hashes[name] = digest(file.body)
	}
	previous, found := c.selections[r.PredecessorBindingSHA256]
	if !found {
		return ErrBinding
	}
	for _, proof := range c.files.recoveredReplacements {
		if proof.original == s.original && proof.beforeReload != nil {
			s.beforeReload = proof
			break
		}
	}
	if s.beforeReload != nil {
		return s.runBeforeReload(bindingPath, previous, hashes)
	}
	beforePath, afterPath := replacementNativePaths(s.original.prefix.journal.path)
	before, after := c.files.files[beforePath], c.files.files[afterPath]
	if before == nil || after == nil {
		return ErrBinding
	}
	completion, err := stoppedReplacementCompletion(s.original, s.candidate, previous, hashes, before.body, after.body)
	if err != nil {
		return err
	}
	s.semantic, err = inspectReplacement(s.ctx, c)
	if err != nil {
		return err
	}
	s.native, err = openRecoveryContext(s.ctx, s.original.cut.Boundary.NativeContext)
	if err != nil {
		return err
	}
	s.manager, err = systemd.OpenForwardingRecoveryReplacement(s.ctx, s.native.expected.Manager)
	if err != nil {
		return err
	}
	root := c.binding.Request.InstallationRoot
	s.publication.originalPending = c.files.files[replacementPending(root)]
	s.publication.originalParent = c.files.dirs[root].info
	if s.publication.originalPending == nil {
		return ErrBinding
	}
	c.files.recoveryPublishing = s.publication
	if err := s.observe(); err != nil {
		return err
	}
	unit := c.files.files["/etc/systemd/system/ardents-forwarding.service"]
	if unit == nil {
		return ErrBinding
	}
	if err := s.publication.create(s.ctx, s.original, completion, c.files.dirs[sealed].info, unit.info, s.observe); err != nil {
		return err
	}
	if err := errors.Join(s.observe(), s.publication.close(), s.manager.Close(), s.native.close(), s.history.close(), s.original.close()); err != nil {
		return err
	}
	// The reader owns new original/cut/suffix leases and native observations;
	// neither the old recovery observer nor writer lends it live custody.
	if err := readbackRecoveryStopped(s.ctx, s.v, s.a, s.input, s.publication, bindingPath); err != nil {
		return err
	}
	if err = errors.Join(s.semantic.check(), s.input.Check(), s.a.Check(s.ctx, s.v)); err != nil {
		return err
	}
	return nil
}

func (s *recoveryReplacementAttempt) observe() error {
	if err := errors.Join(s.ctx.Err(), s.input.Check(), s.a.Check(s.ctx, s.v), s.original.check(), (Started{original: s.history}).Check(), s.semantic.check(), s.native.check()); err != nil {
		return err
	}
	id, err := s.manager.ManagerIdentity()
	if err != nil || id != s.native.expected.Manager {
		return errors.Join(ErrBinding, err)
	}
	h := s.history
	for round := 0; round < 2; round++ {
		if err := cgroup.CheckForwardingQuiescent(s.ctx); err != nil {
			return err
		}
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		if !slices.Equal(facts.UnitPaths, s.native.expected.UnitPaths) {
			return ErrBinding
		}
		var policy error
		if s.beforeReload != nil && s.after == nil {
			previous, ok := h.completed.original.selections[s.candidate.selection.replacement.PredecessorBindingSHA256]
			if !ok {
				return ErrBinding
			}
			policy = verifyForwardingRestartStopped(facts.Unit, facts.Service, previous.binding.Request, previous.binding.Generation, h.snapshot.archive, h.snapshot.id, h.latestRetirement.result.execution())
		} else {
			policy = verifyForwardingReplacementStopped(facts.Unit, facts.Service, s.candidate.selection, h.snapshot.archive, h.snapshot.number, h.snapshot.id, h.latestRetirement.result, h.latestRetirement.sha256)
		}
		if err := policy; err != nil {
			return err
		}
	}
	return errors.Join(s.input.Check(), s.a.Check(s.ctx, s.v), s.original.check(), (Started{original: s.history}).Check(), s.semantic.check(), s.native.check(), s.ctx.Err())
}

func (s *recoveryReplacementAttempt) close() error {
	if s == nil {
		return nil
	}
	s.terminal = errors.Join(s.terminal, s.publication.close(), s.manager.Close(), s.native.close(), s.semantic.close(), s.history.close(), s.ctx.Err())
	clear(s.after)
	return s.terminal
}
