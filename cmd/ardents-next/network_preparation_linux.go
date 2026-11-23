//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

type preparationPath = artifacts.DirectoryChain

func openPreparationParent(path string) (*preparationPath, error) {
	return openPreparationDirectories(context.Background(), path, nil)
}

func openPreparationDirectories(ctx context.Context, path string, owner *uint32) (*preparationPath, error) {
	directories, err := artifacts.OpenDirectoryChain(ctx, path, owner)
	if errors.Is(err, artifacts.ErrInput) {
		return nil, errors.Join(errNetworkPreparation, err)
	}
	return directories, err
}

func readNetworkPreparation(ctx context.Context, path string) (raw []byte, err error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	p, err := openPreparationParent(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, p.Close()) }()
	f, err := p.OpenFile(filepath.Base(path))
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() > maximumNetworkPreparationPlan {
		return nil, errors.Join(errNetworkPreparation, err)
	}
	initial, ok := before.Sys().(*syscall.Stat_t)
	if !ok || initial.Nlink != 1 {
		return nil, errNetworkPreparation
	}
	raw, err = io.ReadAll(io.LimitReader(f, maximumNetworkPreparationPlan+1))
	if err != nil || len(raw) > maximumNetworkPreparationPlan {
		return nil, errors.Join(errNetworkPreparation, err)
	}
	after, e := f.Stat()
	if e != nil || !os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return nil, errors.Join(errNetworkPreparation, e)
	}
	final, ok := after.Sys().(*syscall.Stat_t)
	if !ok || final.Ctim != initial.Ctim || final.Nlink != 1 {
		return nil, errNetworkPreparation
	}
	linked, e := p.OpenFile(filepath.Base(path))
	if e != nil {
		return nil, e
	}
	observed, e := linked.Stat()
	e = errors.Join(e, linked.Close(), p.Check(), ctx.Err())
	if e != nil || !os.SameFile(before, observed) {
		return nil, errors.Join(errNetworkPreparation, e)
	}
	return raw, nil
}

func writeNetworkPreparationReceipt(ctx context.Context, out io.Writer, raw []byte) error {
	return artifacts.WriteReport(ctx, out, raw)
}
