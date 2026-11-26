//go:build linux

package runtime

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/domain/execution/worker"
)

// Fixed bounded document INIT belongs to the document Application. This platform seam performs only that
// destination-free exchange; common launch owns Job, artifact and Grant checks.
func initializeWorker(ctx context.Context, attachment *worker.Attachment, role string, nonce [32]byte, snapshot []byte) error {
	mode := document.ReaderWorker
	if role == "publisher" {
		mode = document.PublisherWorker
	} else if role != "reader" {
		return errors.New("execution worker role is invalid")
	}
	return document.InitializeWorker(ctx, attachment, mode, nonce, snapshot)
}
