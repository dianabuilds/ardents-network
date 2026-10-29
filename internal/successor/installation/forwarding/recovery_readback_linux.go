package forwarding

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Independent completion readback opens all parent/child, kernel and manager
// custody anew after the writer and original observers close. It repeats only
// stopped observations, never Reload, Start, Stop or a domain report.
func readbackRecoveryStopped(ctx context.Context, v *release.Verifier, a infrastructure.ForwardingAuthorization, input Inputs, publication *recoveryPublication, bindingPath string) (returned error) {
	if publication == nil || publication.births == nil || !publication.births.closed || publication.births.terminal != nil {
		return ErrInput
	}
	var pending *initialFile
	if publication.completion.Operation == "replacement" {
		pending = publication.originalPending
	}
	completed, err := readCompletedWithRecovery(ctx, bindingPath, nil, pending, publication)
	if err != nil {
		return err
	}
	c := completed.original
	h := &started{invocationChain: c.files.invocations, ctx: ctx, verifier: v, completed: completed}
	var manager *systemd.ForwardingReplacement
	var startTerminal *startTerminalSnapshot
	defer func() {
		returned = errors.Join(returned, manager.Close(), h.close(), ctx.Err())
		if returned == nil && startTerminal != nil {
			publication.startReadback = startTerminal
		}
	}()
	selected := c.files.files[filepath.Join(c.binding.Request.InstallationRoot, "binding.json")]
	if selected == nil || c.binding.Generation != publication.completion.Generation || digest(selected.body) != publication.completion.BindingSHA256 || c.files.recoveryContext == nil {
		return ErrBinding
	}
	native := c.files.recoveryContext
	check := func() error {
		if err := errors.Join(ctx.Err(), input.Check(), a.Check(ctx, v), completed.CheckAuthorization(v, a), native.check()); err != nil {
			return err
		}
		if publication.completion.Operation == "initial" {
			if h.invocationChain == nil || h.snapshot != nil || len(h.starts) != 0 || len(h.retirements) != 0 {
				return ErrBinding
			}
			if err := checkInitialEmptyRoots(c.files, c.binding.Request); err != nil {
				return err
			}
		} else if err := (Started{original: h}).Check(); err != nil {
			return err
		}
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	manager, err = systemd.OpenForwardingRecoveryReplacement(ctx, native.expected.Manager)
	if err != nil {
		return err
	}
	for round := 0; round < 2; round++ {
		if err := errors.Join(check(), cgroup.CheckForwardingQuiescent(ctx)); err != nil {
			return err
		}
		id, err := manager.ManagerIdentity()
		if err != nil || id != native.expected.Manager {
			return errors.Join(ErrBinding, err)
		}
		facts, err := manager.Observe()
		if err != nil {
			return err
		}
		if !slices.Equal(facts.UnitPaths, native.expected.UnitPaths) {
			return ErrBinding
		}
		switch publication.completion.Operation {
		case "initial":
			err = verifyForwardingStopped(facts.Unit, facts.Service, c.binding.Request, c.binding.Generation)
		case "replacement":
			selected, found := c.selections[publication.completion.BindingSHA256]
			if !found || h.snapshot == nil || h.latestRetirement == nil {
				return ErrBinding
			}
			err = verifyForwardingReplacementStopped(facts.Unit, facts.Service, selected, h.snapshot.archive, h.snapshot.number, h.snapshot.id, h.latestRetirement.result, h.latestRetirement.sha256)
		case "retirement":
			retired := h.latestRetirement
			if retired == nil || h.snapshot == nil || retired.number != h.snapshot.number || retired.journal.path != publication.completion.Journal || retired.binding.Generation != c.binding.Generation {
				return ErrBinding
			}
			err = verifyForwardingRestartStopped(facts.Unit, facts.Service, retired.binding.Request, retired.binding.Generation, retired.archive, retired.id, retired.result.execution())
		case "start":
			head := h.head()
			if head == nil || head.terminal == nil || head.terminal.journal.path != publication.completion.Journal || head.terminal.sha256 != digest(publication.raw) || head.selection.binding.Generation != c.binding.Generation {
				return ErrBinding
			}
			if h.snapshot != nil && id != h.snapshot.archive.Manager {
				return ErrBinding
			}
			startTerminal = head.terminal
			err = verifyNeverStartedStopped(h, facts)
		default:
			return ErrBinding
		}
		if err != nil {
			return err
		}
		if err := errors.Join(check(), cgroup.CheckForwardingQuiescent(ctx)); err != nil {
			return err
		}
	}
	return check()
}

// Initial recovery owns the independently proven native births, before any
// semantic owner has initialized these roots. Recheck actual emptiness during
// fresh readback as well as before writer/old-observer close.
func checkInitialEmptyRoots(files *completedFiles, r Request) error {
	if files == nil {
		return ErrInput
	}
	if err := files.links(); err != nil {
		return err
	}
	for _, path := range []string{r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot} {
		dir := files.dirs[path]
		if dir == nil || dir.file == nil {
			return ErrBinding
		}
		if _, err := dir.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		entries, err := dir.file.ReadDir(1)
		if len(entries) != 0 || err != io.EOF {
			return errors.Join(ErrBinding, err)
		}
	}
	return files.links()
}
