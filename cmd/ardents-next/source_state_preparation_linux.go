package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

func runSourceStatePreparation(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil || len(args) != 2 {
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := admitSourcePreparationPrincipal(); err != nil {
		return sourceStartupFailure(diagnostic, "input", err, nil)
	}
	if err := prepareSourceStateInputs(ctx, args[0], args[1]); err != nil {
		return sourceStartupFailure(diagnostic, "state", err, nil)
	}
	const receipt = "{\"operation\":\"source.prepare-state\",\"outcome\":\"state-prepared\"}\n"
	if writeSourceReportContext(ctx, out, []byte(receipt)) != nil {
		return 1
	}
	return 0
}

func admitSourcePreparationPrincipal() error {
	uid := os.Geteuid()
	if uid < 100 || uid > 999 || os.Getuid() != uid || os.Getgid() != uid || os.Getegid() != uid {
		return errSourceInput
	}
	groups, err := os.Getgroups()
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group != uid {
			return errSourceInput
		}
	}
	return nil
}

func prepareSourceStateInputs(ctx context.Context, planPath, bundlePath string) (result error) {
	if ctx == nil {
		return errSourceInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	custody := sourceStateCustody{files: newSourceInputs(ctx, nil), uid: uint32(os.Geteuid())}
	defer func() { result = errors.Join(result, custody.close(), ctx.Err()) }()
	plan, bundle, err := custody.read(planPath, bundlePath)
	if err != nil {
		return err
	}
	return state.PrepareInitialSource(ctx, plan, bundle, custody.check)
}

// This finite operation borrows the public input mechanism without starting
// runtime supervision. The CLI retains both roots until State has joined.
type sourceStateCustody struct {
	files sourceInputs
	roots []*preparationPath
	uid   uint32
}

func (c *sourceStateCustody) read(planPath, bundlePath string) (plan servingdeclaration.ServingDeclaration, bundle source.Bundle, err error) {
	if planPath == bundlePath {
		return plan, bundle, errSourceInput
	}
	for _, path := range []string{planPath, bundlePath} {
		if path == "/" || len(path) > 4096 || strings.Count(filepath.Dir(path), "/") >= 128 {
			return plan, bundle, errSourceInput
		}
	}
	originalPlan, err := c.files.acquireWithAccess(planPath, 16<<10, sourceStateInput)
	if err != nil {
		return plan, bundle, err
	}
	originalBundle, err := c.files.acquireWithAccess(bundlePath, 1<<20, sourceStateInput)
	if err != nil {
		return plan, bundle, err
	}
	plan, err = servingdeclaration.DecodeServingDeclaration(originalPlan.raw)
	if err != nil || plan.ProcessProfile != processresource.SourceProfile {
		return plan, bundle, errors.Join(errSourceInput, err)
	}
	config, err := state.SourcePreparationConfig(plan)
	if err != nil {
		return plan, bundle, err
	}
	for _, root := range []string{config.Root, plan.ParticipationRoot} {
		for _, path := range []string{planPath, bundlePath} {
			if path == root || strings.HasPrefix(path, root+"/") {
				return plan, bundle, errSourceInput
			}
		}
		if err := c.retainVacantRoot(root); err != nil {
			return plan, bundle, err
		}
	}
	bundle, err = source.DecodeBundle(originalBundle.raw)
	return plan, bundle, err
}

func (c *sourceStateCustody) retainVacantRoot(path string) error {
	if c.files.ctx.Err() != nil || strings.Count(path, "/") >= 128 {
		return errors.Join(errSourceInput, c.files.ctx.Err())
	}
	root, err := openPreparationDirectories(c.files.ctx, path, &c.uid)
	if err != nil {
		return err
	}
	c.roots = append(c.roots, root)
	if err := checkSourceStatePath(root); err != nil {
		return err
	}
	info := root.Info()
	stat := info.Sys().(*syscall.Stat_t)
	if info.Mode().Perm() != 0700 || stat.Uid != c.uid || stat.Gid != c.uid {
		return errSourceInput
	}
	entries, err := root.Descriptor().ReadDir(1)
	if err != io.EOF || len(entries) != 0 {
		return errors.Join(errSourceInput, err)
	}
	return nil
}

func checkSourceStatePath(path *preparationPath) error {
	if err := path.CheckOriginalAccess(); err != nil {
		return errors.Join(errSourceInput, err)
	}
	return nil
}

func (c *sourceStateCustody) check() error {
	if err := c.files.Check(); err != nil {
		return err
	}
	// Reobserve every input after all content reads. A later read cannot carry
	// an earlier descriptor or namespace check through the state handoff.
	for _, input := range c.files.files {
		if err := errors.Join(input.checkAncestors(), input.checkLink(), c.files.ctx.Err()); err != nil {
			return c.files.refuse(err)
		}
	}
	for _, root := range c.roots {
		if err := checkSourceStatePath(root); err != nil {
			return c.files.refuse(err)
		}
	}
	return c.files.ctx.Err()
}

func (c *sourceStateCustody) close() error {
	result := c.check()
	for i := len(c.roots) - 1; i >= 0; i-- {
		result = errors.Join(result, c.roots[i].Close())
	}
	return errors.Join(result, c.files.Close())
}
