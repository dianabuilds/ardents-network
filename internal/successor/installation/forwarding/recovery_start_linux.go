package forwarding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

type recoveryStartAttempt struct {
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

// This continuation describes only stopped, never-started history. Its caller
// still owes the original process cut, actual resources, authority and joins.
func stoppedStartCompletion(original *recoveryOriginal, h *startTerminalSnapshot, directory os.FileInfo) ([]byte, error) {
	if original == nil || h == nil || h.original != original || h.prefix == nil || directory == nil {
		return nil, ErrBinding
	}
	native, ok := infoSys(directory)
	if !ok {
		return nil, ErrBinding
	}
	rows := []startEntry{}
	if _, born := h.prefix.owned[h.path]; !born {
		rows = append(rows, startEntry{Phase: "terminal-directory-intent", Path: h.path}, startEntry{Phase: "terminal-directory-born", Path: h.path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0700}, startEntry{Phase: "terminal-directory-ready", Path: h.path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0750, GID: h.selection.binding.GID})
	}
	rows = append(rows, startEntry{Phase: "stopped-before-start", Path: h.path, Device: uint64(native.Dev), Inode: native.Ino, SHA256: digest(h.prefix.intent), Mode: uint32(directory.Mode().Perm()), UID: native.Uid, GID: native.Gid})
	full := bytes.Clone(original.prefix.journal.body)
	delta := []json.RawMessage{}
	for n, row := range rows {
		row.Schema = "ardents-forwarding-start-journal-v1"
		row.Sequence = uint32(len(h.prefix.rows) + n)
		raw, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		delta = append(delta, raw)
		full = append(full, raw...)
		full = append(full, '\n')
	}
	b := original.cut.Boundary
	completion := recoveryCompletion{Schema: "ardents-forwarding-recovery-completion-v1", Operation: "start", Journal: b.Journal, Device: b.Device, Inode: b.Inode, PrefixSHA256: digest(original.prefix.journal.body), PrefixRecords: b.Records, Generation: b.Generation, BindingSHA256: b.BindingSHA256, OriginalCutSHA256: digest(original.result.body), Outcome: "stopped-before-start", Records: delta, TranscriptSHA256: digest(full)}
	view := *h
	view.directoryInfo = directory
	if err := view.validateContinuation(completion); err != nil {
		return nil, err
	}
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

func (s *recoveryPublication) createStart(ctx context.Context, original *recoveryOriginal, h *startTerminalSnapshot, candidate, unit os.FileInfo, admit func() error) (returned error) {
	if s == nil || original == nil || h == nil || h.original != original {
		return ErrInput
	}
	b := original.cut.Boundary
	s.completion = recoveryCompletion{Operation: "start", Journal: b.Journal, OriginalCutSHA256: digest(original.result.body), BindingSHA256: b.BindingSHA256}
	defer func() {
		if returned != nil {
			returned = errors.Join(returned, s.close())
		}
	}()
	selected, err := s.begin(ctx, original, candidate, unit, admit, h)
	if err != nil {
		return err
	}
	directory := h.directoryInfo
	if _, born := h.prefix.owned[h.path]; !born {
		if err := s.births.directory(h.path, 0750, 0, selected.binding.GID, admit); err != nil {
			return err
		}
		directory = s.births.identities[h.path]
	}
	raw, err := stoppedStartCompletion(original, h, directory)
	if err != nil {
		return err
	}
	completion, _, err := decodeRecoveryCompletion(raw, original.prefix.journal.body, original.result.body, original.cut)
	if err != nil {
		return err
	}
	s.raw, s.completion = raw, completion
	return s.finish(original, selected, candidate, unit, directory, admit)
}

func (s *recoveryStartAttempt) run(bindingPath, historyPath string) error {
	completed, err := readCompletedInterrupted(s.ctx, bindingPath, s.original)
	if err != nil {
		return err
	}
	c := completed.original
	s.history = &started{invocationChain: c.files.invocations, ctx: s.ctx, verifier: s.v, completed: completed}
	if err := errors.Join(completed.CheckHistoryPath(historyPath), completed.CheckAuthorization(s.v, s.a), s.v.CheckHistoryPath(s.ctx, historyPath), (Started{original: s.history}).Check()); err != nil {
		return err
	}
	head := s.history.head()
	if head == nil || head.terminal == nil || head.terminal.original != s.original || head.terminal.journal.path != s.original.cut.Boundary.Journal || head.selection.binding.Generation != c.binding.Generation {
		return ErrBinding
	}
	h := head.terminal
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
	s.semantic, err = inspectStartRecovery(s.ctx, c, h)
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
	s.publication.originalPending, s.publication.originalParent = h.pending, h.directoryInfo
	c.files.recoveryPublishing = s.publication
	if err := s.observe(); err != nil {
		return err
	}
	unit := c.files.files["/etc/systemd/system/ardents-forwarding.service"]
	if unit == nil || c.files.dirs[sealed] == nil {
		return ErrBinding
	}
	if err := s.publication.createStart(s.ctx, s.original, h, c.files.dirs[sealed].info, unit.info, s.observe); err != nil {
		return err
	}
	if err := errors.Join(s.observe(), s.publication.close(), s.manager.Close(), s.native.close(), s.history.close(), s.original.close()); err != nil {
		return err
	}
	if err := readbackRecoveryStopped(s.ctx, s.v, s.a, s.input, s.publication, bindingPath); err != nil {
		return err
	}
	return errors.Join(s.semantic.check(), s.input.Check(), s.a.Check(s.ctx, s.v))
}

func verifyNeverStartedStopped(h *started, facts systemd.ForwardingObservation) error {
	if h == nil || h.invocationChain == nil || h.head() == nil || h.head().terminal == nil {
		return ErrBinding
	}
	selected := h.head().selection
	if h.snapshot == nil {
		if len(h.starts) != 0 || len(h.retirements) != 0 {
			return ErrBinding
		}
		return verifyForwardingStopped(facts.Unit, facts.Service, selected.binding.Request, selected.binding.Generation)
	}
	r := h.latestRetirement
	if r == nil || r.number != h.snapshot.number {
		return ErrBinding
	}
	if selected.binding.Generation == h.snapshot.archive.Generation {
		return verifyForwardingRestartStopped(facts.Unit, facts.Service, selected.binding.Request, selected.binding.Generation, h.snapshot.archive, h.snapshot.id, r.result.execution())
	}
	return verifyForwardingReplacementStopped(facts.Unit, facts.Service, selected, h.snapshot.archive, h.snapshot.number, h.snapshot.id, r.result, r.sha256)
}

func (s *recoveryStartAttempt) observe() error {
	check := func() error {
		return errors.Join(s.ctx.Err(), s.input.Check(), s.a.Check(s.ctx, s.v), s.original.check(), (Started{original: s.history}).Check(), s.semantic.check(), s.native.check())
	}
	if err := check(); err != nil {
		return err
	}
	id, err := s.manager.ManagerIdentity()
	if err != nil || id != s.native.expected.Manager {
		return errors.Join(ErrBinding, err)
	}
	if s.history.snapshot != nil && id != s.history.snapshot.archive.Manager {
		return ErrBinding
	}
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
		if err := verifyNeverStartedStopped(s.history, facts); err != nil {
			return err
		}
		if err := errors.Join(check(), cgroup.CheckForwardingQuiescent(s.ctx)); err != nil {
			return err
		}
	}
	return check()
}

func (s *recoveryStartAttempt) close() error {
	if s == nil {
		return nil
	}
	s.terminal = errors.Join(s.terminal, s.publication.close(), s.manager.Close(), s.native.close(), s.semantic.close(), s.history.close(), s.ctx.Err())
	return s.terminal
}

// Only a successfully closed independent readback supplies this proof. A nil
// original pending is not a generic publication waiver.
func publishNeverStartedAbsentPending(s *recoveryPublication) error {
	if s == nil || s.completion.Operation != "start" || s.originalPending != nil || s.births == nil || !s.births.closed || s.births.terminal != nil || s.pending == nil {
		return ErrRepairRequired
	}
	h := s.startReadback
	if h == nil || !h.closed || h.terminal != nil || h.original != nil || h.prefix == nil || h.sha256 != digest(s.raw) || h.guard == nil || !sameInputFile(h.guard.info, s.pending.info) || h.journal.path != s.completion.Journal || h.directoryInfo == nil {
		return ErrBinding
	}
	path := filepath.Join(h.path, "forwarding-start-pending")
	if _, born := h.prefix.owned[path]; born {
		return ErrBinding
	}
	if err := s.births.ctx.Err(); err != nil {
		return err
	}
	guard, err := os.Lstat(s.pending.path)
	if err != nil || !sameInputFile(guard, s.pending.info) {
		return errors.Join(ErrBinding, err)
	}
	parent, err := os.Lstat(h.path)
	if err != nil || !sameInputFile(parent, h.directoryInfo) {
		return errors.Join(ErrBinding, err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(ErrBinding, err)
	}
	return s.births.ctx.Err()
}
