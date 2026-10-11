//go:build linux

package runtime

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/successor/execution/worker"
)

// Fixed bounded INIT belongs to Text. This platform seam performs only that
// destination-free exchange; common launch owns Job, artifact and Grant checks.
func initializeWorker(ctx context.Context, attachment *worker.Attachment, role string, nonce [32]byte, snapshot []byte) error {
	mode := textdocument.ReaderWorker
	if role == "publisher" {
		mode = textdocument.PublisherWorker
	} else if role != "reader" {
		return errors.New("execution worker role is invalid")
	}
	return textdocument.InitializeWorker(ctx, attachment, mode, nonce, snapshot)
}
