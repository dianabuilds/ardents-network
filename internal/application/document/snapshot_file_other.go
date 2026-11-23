//go:build !linux

package document

import (
	"context"
	"errors"
)

// ReadSnapshotFile is unavailable outside the selected Linux import boundary.
func ReadSnapshotFile(context.Context, string) ([]byte, error) {
	return nil, errors.New("document snapshot import requires the selected Linux platform")
}
