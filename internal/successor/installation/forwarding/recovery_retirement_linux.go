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

type recoveryRetirementAttempt struct {
	ctx         context.Context
	v           *release.Verifier
	a           infrastructure.ForwardingAuthorization
	input       Inputs
	original    *recoveryOriginal
	candidate   *recoveryCandidate
	history     *started
	semantic    *replacementInspections
	native      *recoveryContext
	manager     *systemd.ForwardingReplacement
	publication *recoveryPublication
	terminal    error
}

func stoppedRetirementCompletion(original *recoveryOriginal, candidate *recoveryCandidate, h *retirementSnapshot) ([]byte, error) {
	logical, err := stoppedRetirementTranscript(original.prefix.journal.body, original.cut, candidate.selection, h.number, h.startSHA, h.archive, h.id, h.files[filepath.Join(h.path, "retirement-result.json")].body)
	if err != nil {
		return nil, err
	}
	boundary := original.cut.Boundary
	delta := []json.RawMessage{}
	if boundary.Records == 17 {
		lines := bytes.Split(logical[:len(logical)-1], []byte{'\n'})
		delta = append(delta, bytes.Clone(lines[17]))
	}
	completion := recoveryCompletion{Schema: "ardents-forwarding-recovery-completion-v1", Operation: "retirement", Journal: boundary.Journal, Device: boundary.Device, Inode: boundary.Inode,
		PrefixSHA256: digest(original.prefix.journal.body), PrefixRecords: boundary.Records, Generation: candidate.selection.binding.Generation, BindingSHA256: digest(candidate.selection.raw),
		OriginalCutSHA256: digest(original.result.body), Outcome: "retired-stopped", Records: delta, TranscriptSHA256: digest(logical)}
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

func (s *recoveryRetirementAttempt) run(bindingPath, historyPath string) error {
	completed, err := readCompletedInterrupted(s.ctx, bindingPath, s.original)
	if err != nil {
		return err
	}
	s.history = &started{invocationChain: completed.original.files.invocations, ctx: s.ctx, verifier: s.v, completed: completed}
	c := completed.original
	if err := errors.Join(completed.CheckHistoryPath(historyPath), completed.CheckAuthorization(s.v, s.a), s.v.CheckHistoryPath(s.ctx, historyPath), (Started{original: s.history}).Check()); err != nil {
		return err
	}
	h := s.history.latestRetirement
	if h == nil || s.history.snapshot == nil || h.journal.path != s.original.cut.Boundary.Journal || h.number != s.history.snapshot.number || h.binding.Generation != c.binding.Generation || h.recovery == nil || h.recovery.original != s.original {
		return ErrBinding
	}
	if err := c.readHostingPreparation(bindingPath); err != nil {
		return err
	}
	sealed := filepath.Join(c.binding.Request.InstallationRoot, "generations", c.binding.Generation)
	for name, raw := range s.candidate.files {
		file := c.files.files[filepath.Join(sealed, name)]
		if file == nil || !bytes.Equal(file.body, raw) {
			return ErrBinding
		}
	}
	completion, err := stoppedRetirementCompletion(s.original, s.candidate, h)
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
	s.publication.originalPending = h.pending
	s.publication.originalParent = h.directoryInfo
	if h.pending == nil {
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
	if err := readbackRecoveryStopped(s.ctx, s.v, s.a, s.input, s.publication, bindingPath); err != nil {
		return err
	}
	if err = errors.Join(s.semantic.check(), s.input.Check(), s.a.Check(s.ctx, s.v)); err != nil {
		return err
	}
	return nil
}

func (s *recoveryRetirementAttempt) observe() error {
	if err := errors.Join(s.ctx.Err(), s.input.Check(), s.a.Check(s.ctx, s.v), s.original.check(), (Started{original: s.history}).Check(), s.semantic.check(), s.native.check()); err != nil {
		return err
	}
	id, err := s.manager.ManagerIdentity()
	if err != nil || id != s.native.expected.Manager {
		return errors.Join(ErrBinding, err)
	}
	h := s.history.latestRetirement
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
		if err := verifyForwardingRestartStopped(facts.Unit, facts.Service, h.binding.Request, h.binding.Generation, h.archive, h.id, h.result.execution()); err != nil {
			return err
		}
	}
	return errors.Join(s.input.Check(), s.a.Check(s.ctx, s.v), s.original.check(), (Started{original: s.history}).Check(), s.semantic.check(), s.native.check(), s.ctx.Err())
}

func (s *recoveryRetirementAttempt) close() error {
	if s == nil {
		return nil
	}
	s.terminal = errors.Join(s.terminal, s.publication.close(), s.manager.Close(), s.native.close(), s.semantic.close(), s.history.close(), s.ctx.Err())
	return s.terminal
}
