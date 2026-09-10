package main

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev1/administration"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

// publishText imports in the trusted owner's process and sends only the
// resulting snapshot to the separately authorized Administration socket.
func publishText(ctx context.Context, socket, path string) error {
	if ctx == nil || !filepath.IsAbs(socket) || !filepath.IsAbs(path) {
		return errTextInput
	}
	snapshot, err := textdocument.ReadSnapshotFile(ctx, path)
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
		return errors.New("text publication did not commit")
	}
	return nil
}
