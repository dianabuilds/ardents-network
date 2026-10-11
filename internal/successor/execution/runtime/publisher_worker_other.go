//go:build !linux

package runtime

import (
	"context"
	"errors"
)

func (*Owner) LaunchPublisher(context.Context, [32]byte, []byte) (*Invocation, error) {
	return nil, errors.New("ordinary Publisher native launch is unavailable")
}

func (invocation *Invocation) observePublisherAttachment() {
	invocation.job.Retire()
	close(invocation.observationDone)
}
