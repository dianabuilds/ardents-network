//go:build !linux

package runtime

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/execution/worker"
)

func (documentProtocol) initialize(context.Context, *worker.Attachment, [32]byte, string) error {
	return errors.New("ordinary Execution native initialization is unavailable")
}
