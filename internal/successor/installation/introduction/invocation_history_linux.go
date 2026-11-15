package introduction

import (
	"errors"
	"os"
	"path/filepath"
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
		if e.Name() == "introduction-startup" || strings.HasPrefix(e.Name(), "introduction-invocation-") {
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
		b := c.binding
		bound := c.files[filepath.Join(root, "binding.json")].body
		snap, err := readStartSnapshotAt(c.ctx, b, n, digest(bound), digest(c.journal.body), previous)
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
		retired, err := readRetirementSnapshot(c.ctx, b, n, snap.bindingSHA, snap.sha256, snap.archive, snap.id, retirementCommitted)
		h.retirements = append(h.retirements, retired)
		if err != nil {
			return err
		}
		previous = retired.sha256
	}
	c.started = h.starts[len(h.starts)-1]
	if len(h.retirements) == len(h.starts) {
		c.retired = h.retirements[len(h.retirements)-1]
	}
	return h.check()
}
