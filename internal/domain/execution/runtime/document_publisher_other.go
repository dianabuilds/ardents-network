//go:build !linux

package runtime

import (
	"context"
	"errors"
	"io"
)

func (*Owner) LaunchPublisher(context.Context, [32]byte, []byte) (*Invocation, error) {
	return nil, errors.New("ordinary Publisher native launch is unavailable")
}

func (*Owner) LaunchSnapshotPublisher(context.Context, [32]byte, io.ReaderAt, int64) (*Invocation, error) {
	return nil, errors.New("snapshot Publisher native launch is unavailable")
}
