//go:build !linux

package runtime

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/execution/worker"
)

func initializeWorker(context.Context, *worker.Attachment, string, [32]byte, []byte) error {
	return errors.New("ordinary Execution native initialization is unavailable")
}
