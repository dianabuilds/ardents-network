//go:build !linux

package runtime

import (
	"context"
	"errors"
)

func (*Owner) LaunchReader(context.Context, [32]byte) (*Invocation, error) {
	return nil, errors.New("ordinary Reader native launch is unavailable")
}

func (invocation *Invocation) observeReaderAttachment() {
	invocation.job.Retire()
	close(invocation.observationDone)
}
