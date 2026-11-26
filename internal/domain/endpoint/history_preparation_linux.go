//go:build linux

package endpoint

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/network/duty"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
)

// PrepareHistories explicitly prepares the first signed State, participation,
// Entry and potential-spend histories in four existing vacant private directories. It
// authenticates public State/profile bytes through Network and creates no
// permission, key, Service or installed startup. It is never recovery:
// every partial or completed earlier attempt is preserved and
// refuses a second preparation. Hosting and Service initialization are separate.
func PrepareHistories(ctx context.Context, plan HeadlessPlan, bundle source.Bundle, profile []byte) error {
	return prepareHistories(ctx, plan, bundle, profile, nil)
}

// afterState only schedules interruption after the actual exclusive State
// claim in tests. It cannot provide successful storage, authority or startup.
func prepareHistories(ctx context.Context, plan HeadlessPlan, bundle source.Bundle, profile []byte, afterState func()) (result error) {
	if ctx == nil || plan.Schema != "ardents-headless-runtime-v2" || plan.Role != "" && plan.Role != "reader" {
		return errors.New("invalid participant history preparation")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	config, err := plan.Network()
	if err != nil {
		return err
	}
	paths := []string{plan.NetworkStateRoot, plan.LocalRoleStateRoot, plan.ApplicationTokenRoot, plan.EntryStateRoot}
	// These are only preparation targets, not an accepting runtime-plan check.
	// They must not overlap one another or the declared independent clock file.
	isolated := append(append([]string(nil), paths...), plan.TimeConfidenceFile)
	for i, path := range isolated {
		if !pathgrammar.Strict(path) || path == "/" {
			return errors.New("invalid participant history path")
		}
		for _, previous := range isolated[:i] {
			if path == previous || strings.HasPrefix(path, previous+"/") || strings.HasPrefix(previous, path+"/") {
				return errors.New("participant history paths overlap")
			}
		}
	}
	var roots []*os.File
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			result = errors.Join(result, roots[i].Close())
		}
		result = errors.Join(result, ctx.Err())
	}()
	for _, path := range paths {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != path {
			return errors.Join(errors.New("participant preparation requires direct roots"), err)
		}
		root, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
		if err != nil {
			return err
		}
		roots = append(roots, root)
		info, err := root.Stat()
		if err != nil || info.Mode().Perm() != 0700 {
			return errors.Join(errors.New("participant preparation requires private directories"), err)
		}
		status, ok := info.Sys().(*syscall.Stat_t)
		if !ok || status.Uid != uint32(os.Geteuid()) || status.Gid != uint32(os.Getegid()) {
			return errors.New("participant history has a foreign principal")
		}
		entries, err := root.ReadDir(1)
		if err != io.EOF || len(entries) != 0 {
			return errors.Join(errors.New("participant preparation requires vacant directories"), err)
		}
	}
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		for i, root := range roots {
			original, err := root.Stat()
			if err != nil {
				return err
			}
			current, err := os.Lstat(paths[i])
			if err != nil || !os.SameFile(original, current) || !current.IsDir() || current.Mode().Perm() != 0700 {
				return errors.Join(errors.New("participant preparation root changed"), err)
			}
			status, ok := current.Sys().(*syscall.Stat_t)
			resolved, resolveErr := filepath.EvalSymlinks(paths[i])
			if !ok || status.Uid != uint32(os.Geteuid()) || status.Gid != uint32(os.Getegid()) || resolveErr != nil || resolved != paths[i] {
				return errors.Join(errors.New("participant preparation custody changed"), resolveErr)
			}
		}
		return nil
	}
	config.PermitWork = check
	// This exclusive fresh State claim fences concurrent/second preparation
	// through all later effects and joins. There is no aggregate success marker
	// or rollback: each domain retains its own completed or incomplete history.
	network, err := state.OpenFresh(config)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, network.Close(), check()) }()
	if afterState != nil {
		afterState()
	}
	if err := check(); err != nil {
		return err
	}
	// A fresh but unauthenticated State is incomplete and cannot be reopened
	// retained. Never relax that requirement to manufacture first-start success.
	if _, err := network.AcceptEpoch(ctx, bundle.Epoch, bundle.Inputs, bundle.Materials); err != nil {
		return err
	}
	if _, err := network.AcceptClosedProfile(profile); err != nil {
		return err
	}
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		view, err := network.CurrentRuntime()
		if err != nil {
			return admission.AuthorityFacts{}, time.Time{}, err
		}
		return funding.NetworkFacts(view)
	}
	if err := duty.Initialize(ctx, duty.Config{Root: plan.LocalRoleStateRoot, Clock: time.Now}); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	entryRoot := filepath.Join(plan.EntryStateRoot, "entry")
	roles := []selection.RoleConfig{{InteriorRoot: filepath.Join(plan.EntryStateRoot, "reader-interior"), Domain: 1}}
	if plan.Role == "" {
		roles = append(roles,
			selection.RoleConfig{InteriorRoot: filepath.Join(plan.EntryStateRoot, "introduction-interior"), Domain: 4},
			selection.RoleConfig{InteriorRoot: filepath.Join(plan.EntryStateRoot, "publisher-source-interior"), Domain: 1},
			selection.RoleConfig{InteriorRoot: filepath.Join(plan.EntryStateRoot, "responder-interior"), Domain: 3})
	}
	selectionPaths := []string{entryRoot}
	for _, role := range roles {
		selectionPaths = append(selectionPaths, role.InteriorRoot)
	}
	for _, path := range selectionPaths {
		if err := os.Mkdir(path, 0700); err != nil {
			return err
		}
	}
	if err := roots[3].Sync(); err != nil {
		return err
	}
	if err := selection.InitializeInstallation(selection.InstallationConfig{EntryRoot: entryRoot, Current: network.CurrentRuntime}, roles...); err != nil {
		return err
	}
	names := []string{"reader"}
	if plan.Role == "" {
		names = append(names, "publisher")
	}
	for _, name := range names {
		if err := check(); err != nil {
			return err
		}
		path := filepath.Join(plan.ApplicationTokenRoot, name)
		// Exclusive child birth is the only fresh presentation-journal caller.
		// An existing, missing-after-restart or partial child is never adopted.
		if err := os.Mkdir(path, 0700); err != nil {
			return err
		}
		if err := roots[2].Sync(); err != nil {
			return err
		}
		role := admission.AllocationUser
		if name == "publisher" {
			role = admission.AllocationPublisher
		}
		holder, err := stock.Open(path, role, observe)
		if err != nil {
			return err
		}
		if err := errors.Join(holder.Close(), check()); err != nil {
			return err
		}
	}
	return check()
}
