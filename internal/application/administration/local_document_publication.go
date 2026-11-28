package administration

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	"github.com/dianabuilds/ardents-network/internal/application/localsnapshot"
)

// PublishLocal imports an explicit snapshot and transfers its bytes through the
// separately authorized Administration connection. It confirms the input after
// two reads and returns only once the publication commits.
func PublishLocal(ctx context.Context, socket, path string) error {
	if ctx == nil || !filepath.IsAbs(socket) || !filepath.IsAbs(path) {
		return endpointstream.ErrInvalidLocalInput
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	outcome, err := requestSnapshot(bounded, socket, func(output io.Writer) error {
		return localsnapshot.CopyVerified(bounded, path, MaximumSnapshotBytes, func(size int64) (io.Writer, error) {
			if err := writeSnapshotHeader(output, size); err != nil {
				return nil, err
			}
			return output, nil
		})
	})
	if err != nil {
		return errors.Join(err, bounded.Err())
	}
	if outcome != Published {
		return errors.New("document publication did not commit")
	}
	return nil
}
