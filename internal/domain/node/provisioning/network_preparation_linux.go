//go:build linux

package provisioning

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// PrepareNetworkArtifact writes one unsigned artifact from the original public
// plan. It creates no signed State or runtime authority. The input reader uses
// the Unix-family inspection port, but publication composes the Linux ext4
// output floor (artifacts.PublishUnsigned), so the operation keeps its Linux
// boundary until a macOS storage floor is selected (portability audit #848).
func PrepareNetworkArtifact(ctx context.Context, operation, input, output string) (PreparedNetworkArtifact, error) {
	if ctx == nil || !IsNetworkPreparation(operation) {
		return PreparedNetworkArtifact{}, errNetworkPreparation
	}
	raw, err := readNetworkPreparation(ctx, input)
	if err != nil || len(raw) > MaximumNetworkPreparationPlan {
		return PreparedNetworkArtifact{}, errors.Join(errNetworkPreparation, err)
	}
	name, message, err := PrepareNetworkPublic(operation, raw)
	if err != nil {
		return PreparedNetworkArtifact{}, err
	}
	if err := PublishPreparedNetworkArtifact(ctx, output, name, message, nil); err != nil {
		return PreparedNetworkArtifact{}, errors.Join(ErrNetworkPreparationOutput, err)
	}
	return PreparedNetworkArtifact{Name: name, Digest: sha256.Sum256(message)}, nil
}

func readNetworkPreparation(ctx context.Context, path string) (raw []byte, err error) {
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
	if err != nil || !before.Mode().IsRegular() || before.Size() > MaximumNetworkPreparationPlan {
		return nil, errors.Join(errNetworkPreparation, err)
	}
	initial, ok := before.Sys().(*syscall.Stat_t)
	if !ok || initial.Nlink != 1 {
		return nil, errNetworkPreparation
	}
	raw, err = io.ReadAll(io.LimitReader(f, MaximumNetworkPreparationPlan+1))
	if err != nil || len(raw) > MaximumNetworkPreparationPlan {
		return nil, errors.Join(errNetworkPreparation, err)
	}
	after, e := f.Stat()
	if e != nil || !os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return nil, errors.Join(errNetworkPreparation, e)
	}
	final, ok := after.Sys().(*syscall.Stat_t)
	if !ok || final.Nlink != 1 {
		return nil, errNetworkPreparation
	}
	initialCtime, initialOK := durablefile.InodeChangeTime(before)
	finalCtime, finalOK := durablefile.InodeChangeTime(after)
	if !initialOK || !finalOK || finalCtime != initialCtime {
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
