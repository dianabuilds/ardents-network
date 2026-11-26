//go:build linux

package state

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/platform/process"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

// ErrSourcePreparationPrincipal identifies refusal before opening any input.
var ErrSourcePreparationPrincipal = errors.New("source State preparation requires the fixed service principal")

// PrepareSourceStateFiles initializes signed State and empty participation as
// the fixed service principal. Original files and both directories are held
// through the State and participation operations and checked before closure.
func PrepareSourceStateFiles(ctx context.Context, planPath, bundlePath string) error {
	if err := process.RequireFixedServicePrincipal(); err != nil {
		return errors.Join(ErrSourcePreparationPrincipal, err)
	}
	return prepareSourceStateFiles(ctx, planPath, bundlePath)
}

func prepareSourceStateFiles(ctx context.Context, planPath, bundlePath string) (result error) {
	if ctx == nil {
		return errSourcePreparation
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	originals := artifacts.NewInputs(ctx)
	defer func() { result = errors.Join(result, originals.Close(), ctx.Err()) }()
	preparation, err := readSourceStateInputs(originals, planPath, bundlePath)
	if err != nil {
		return err
	}
	return preparation.prepare(ctx, originals.Check)
}

type sourceStatePreparation struct {
	config        Config
	participation string
	bundle        source.Bundle
}

func readSourceStateInputs(originals *artifacts.Inputs, planPath, bundlePath string) (result sourceStatePreparation, err error) {
	if planPath == bundlePath {
		return result, errSourcePreparation
	}
	for _, path := range []string{planPath, bundlePath} {
		if !artifacts.AbsolutePath(path) || strings.Count(filepath.Dir(path), "/") >= 128 {
			return result, errSourcePreparation
		}
	}
	planBytes, err := originals.Read(planPath, 16<<10)
	if err != nil {
		return result, err
	}
	bundleBytes, err := originals.Read(bundlePath, 1<<20)
	if err != nil {
		return result, err
	}
	plan, err := servingdeclaration.DecodeServingDeclaration(planBytes)
	if err != nil || plan.ProcessProfile != processresource.SourceProfile {
		return result, errors.Join(errSourcePreparation, err)
	}
	result.config, err = SourcePreparationConfig(plan)
	if err != nil {
		return result, err
	}
	result.participation = plan.ParticipationRoot
	uid := uint32(os.Geteuid())
	for _, path := range []string{result.config.Root, result.participation} {
		for _, input := range []string{planPath, bundlePath} {
			if input == path || strings.HasPrefix(input, path+string(filepath.Separator)) {
				return result, errSourcePreparation
			}
		}
		root, err := originals.Directory(path)
		if err != nil {
			return result, err
		}
		info := root.Info()
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || info.Mode().Perm() != 0700 || stat.Uid != uid || stat.Gid != uid {
			return result, errSourcePreparation
		}
		entries, err := root.Descriptor().ReadDir(1)
		if err != io.EOF || len(entries) != 0 {
			return result, errors.Join(errSourcePreparation, err)
		}
	}
	result.bundle, err = source.DecodeBundle(bundleBytes)
	return result, err
}
