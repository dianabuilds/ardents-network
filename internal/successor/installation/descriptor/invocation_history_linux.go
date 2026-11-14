package descriptor

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	"os"
	"strings"
)

// invocationHistory owns only independently read complete original journals and
// their leases. No live domain/root/manager/process object crosses generations.
type invocationHistory struct {
	starts      []*startSnapshot
	retirements []*retirementSnapshot
}

func (h *invocationHistory) close() error {
	if h == nil {
		return nil
	}
	var e error
	for i := len(h.starts) - 1; i >= 0; i-- {
		if i < len(h.retirements) {
			e = errors.Join(e, h.retirements[i].close())
		}
		e = errors.Join(e, h.starts[i].close())
	}
	return e
}
func (h *invocationHistory) check() error {
	if h == nil {
		return nil
	}
	for i, s := range h.starts {
		if err := s.check(); err != nil {
			return err
		}
		if i < len(h.retirements) {
			if err := h.retirements[i].check(); err != nil {
				return err
			}
		}
	}
	return nil
}
func (h *invocationHistory) owns(path string) bool {
	if h == nil {
		return false
	}
	for _, s := range h.starts {
		if path == s.journal.path {
			return true
		}
	}
	for _, s := range h.retirements {
		if path == s.journal.path {
			return true
		}
	}
	return false
}
func invocationJournal(root, path string) bool {
	for n := attemptNumber(0); n < attemptLimit; n++ {
		if path == n.journal(root) || path == n.retirementJournal(root) {
			return true
		}
	}
	return false
}
func (c *completedInitial) readInvocations() error {
	root := c.binding.Request.InstallationRoot
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) >= historyDirectoryLimit {
		return errors.Join(ErrBinding, err)
	}
	var names []string
	for _, e := range entries {
		if e.Name() == "descriptor-startup" || strings.HasPrefix(e.Name(), "descriptor-invocation-") {
			if !e.IsDir() || e.Type()&os.ModeSymlink != 0 {
				return ErrBinding
			}
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return nil
	}
	latest, err := attemptDirectories(names)
	if err != nil {
		return err
	}
	c.invocations = &invocationHistory{}
	h := c.invocations
	previous := ""
	seen := map[[16]byte]bool{}
	for n := attemptNumber(0); n <= latest; n++ {
		selected := c.selectionForAttempt(n)
		snap, err := readStartSnapshotAt(c.ctx, selected.binding, n, digest(selected.raw), digest(c.journal.body), previous)
		h.starts = append(h.starts, snap)
		if err != nil {
			return err
		}
		if seen[snap.id] {
			return ErrBinding
		}
		seen[snap.id] = true
		_, err = os.Lstat(n.retirement(root))
		if errors.Is(err, os.ErrNotExist) {
			if n != latest {
				return ErrBinding
			}
			break
		}
		if err != nil {
			return err
		}
		retired, err := readRetirementSnapshot(c.ctx, selected.binding, n, snap.bindingSHA, snap.sha256, snap.archive, snap.id, retirementCommitted)
		h.retirements = append(h.retirements, retired)
		if err != nil {
			return err
		}
		previous = retired.sha256
	}
	return h.check()
}

// Started keeps initial/Hosting and all child history under one original initial
// lease. Release expiry cannot prevent Stop; Restart separately checks fresh permission.
type Started struct{ original *started }
type started struct {
	ctx        context.Context
	verifier   *release.Verifier
	completed  Completed
	snapshot   *startSnapshot
	retirement *retirementSnapshot
}

func ReadStarted(ctx context.Context, v *release.Verifier, bindingPath, history string) (result Started, returned error) {
	if ctx == nil || v == nil || os.Geteuid() != 0 || !validPath(history) {
		return result, ErrInput
	}
	c, err := ReadCompleted(ctx, bindingPath)
	if err != nil {
		return result, err
	}
	s := &started{ctx: ctx, verifier: v, completed: c}
	result = Started{s}
	defer func() {
		if returned != nil {
			returned = errors.Join(returned, result.Close())
			result = Started{}
		}
	}()
	if c.original.binding.History != history {
		return result, ErrBinding
	}
	if err := v.CheckHistoryPath(ctx, history); err != nil {
		return result, err
	}
	h := c.original.invocations
	if h == nil || len(h.starts) == 0 {
		return result, ErrBinding
	}
	s.snapshot = h.starts[len(h.starts)-1]
	if len(h.retirements) == len(h.starts) {
		s.retirement = h.retirements[len(h.retirements)-1]
	}
	return result, result.Check()
}
func (h Started) Check() error {
	if h.original == nil {
		return ErrInput
	}
	s := h.original
	if s.verifier == nil || s.ctx == nil {
		return ErrInput
	}
	return s.completed.operationObserved(func() error { return s.verifier.CheckHistoryPath(s.ctx, s.completed.original.binding.History) }, nil)
}
func (h Started) Close() error {
	if h.original == nil {
		return nil
	}
	return h.original.completed.Close()
}
