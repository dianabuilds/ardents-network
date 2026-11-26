//go:build linux

package networkpreparation

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	"github.com/dianabuilds/ardents-network/internal/durablefile"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// Prepare writes one unsigned artifact from the original public
// plan. It creates no signed State or runtime authority. The input reader uses
// the Unix-family inspection port, but publication composes the Linux ext4
// output floor (artifacts.PublishUnsigned), so the operation keeps its Linux
// boundary until a macOS storage floor is selected (portability audit #848).
func Prepare(ctx context.Context, operation, input, output string) (Artifact, error) {
	if ctx == nil || !networkdraft.IsPreparation(operation) {
		return Artifact{}, networkdraft.ErrUnavailable
	}
	raw, err := readDeclaration(ctx, input)
	if err != nil || len(raw) > networkdraft.MaximumPlanBytes {
		return Artifact{}, errors.Join(networkdraft.ErrUnavailable, err)
	}
	name, message, err := networkdraft.Prepare(operation, raw)
	if err != nil {
		return Artifact{}, err
	}
	if err := publish(ctx, output, name, message, nil); err != nil {
		return Artifact{}, errors.Join(ErrOutput, err)
	}
	return Artifact{Name: name, Digest: sha256.Sum256(message)}, nil
}

func readDeclaration(ctx context.Context, path string) (raw []byte, err error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	p, err := artifacts.OpenDirectoryChain(context.Background(), filepath.Dir(path), nil)
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
	if err != nil || !before.Mode().IsRegular() || before.Size() > networkdraft.MaximumPlanBytes {
		return nil, errors.Join(networkdraft.ErrUnavailable, err)
	}
	initial, ok := before.Sys().(*syscall.Stat_t)
	if !ok || initial.Nlink != 1 {
		return nil, networkdraft.ErrUnavailable
	}
	raw, err = io.ReadAll(io.LimitReader(f, networkdraft.MaximumPlanBytes+1))
	if err != nil || len(raw) > networkdraft.MaximumPlanBytes {
		return nil, errors.Join(networkdraft.ErrUnavailable, err)
	}
	after, e := f.Stat()
	if e != nil || !os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return nil, errors.Join(networkdraft.ErrUnavailable, e)
	}
	final, ok := after.Sys().(*syscall.Stat_t)
	if !ok || final.Nlink != 1 {
		return nil, networkdraft.ErrUnavailable
	}
	initialCtime, initialOK := durablefile.InodeChangeTime(before)
	finalCtime, finalOK := durablefile.InodeChangeTime(after)
	if !initialOK || !finalOK || finalCtime != initialCtime {
		return nil, networkdraft.ErrUnavailable
	}
	linked, e := p.OpenFile(filepath.Base(path))
	if e != nil {
		return nil, e
	}
	observed, e := linked.Stat()
	e = errors.Join(e, linked.Close(), p.Check(), ctx.Err())
	if e != nil || !os.SameFile(before, observed) {
		return nil, errors.Join(networkdraft.ErrUnavailable, e)
	}
	return raw, nil
}
