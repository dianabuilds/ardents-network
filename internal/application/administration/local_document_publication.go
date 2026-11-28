package administration

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	"github.com/dianabuilds/ardents-network/internal/application/localsnapshot"
)

// PublishLocal imports an explicit snapshot and transfers its bytes through the
// separately authorized Administration connection. It clears the local copy
// after the bounded request and returns only once the publication commits.
func PublishLocal(ctx context.Context, socket, path string) error {
	if ctx == nil || !filepath.IsAbs(socket) || !filepath.IsAbs(path) {
		return endpointstream.ErrInvalidLocalInput
	}
	snapshot, err := localsnapshot.Read(ctx, path)
	if err != nil {
		return err
	}
	defer clear(snapshot)
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	outcome, err := RequestSnapshot(bounded, socket, snapshot)
	if err != nil {
		return errors.Join(err, bounded.Err())
	}
	if outcome != Published {
		return errors.New("document publication did not commit")
	}
	return nil
}
