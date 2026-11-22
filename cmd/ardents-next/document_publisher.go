package main

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	"github.com/dianabuilds/ardents-network/internal/application/document"
)

// publishDocument imports in the trusted owner's process and sends only the
// resulting snapshot to the separately authorized Administration socket.
func publishDocument(ctx context.Context, socket, path string) error {
	if ctx == nil || !filepath.IsAbs(socket) || !filepath.IsAbs(path) {
		return errDocumentInput
	}
	snapshot, err := document.ReadSnapshotFile(ctx, path)
	if err != nil {
		return err
	}
	defer clear(snapshot)
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	outcome, err := administration.RequestSnapshot(bounded, socket, snapshot)
	if err != nil {
		return errors.Join(err, bounded.Err())
	}
	if outcome != administration.Published {
		return errors.New("document publication did not commit")
	}
	return nil
}
