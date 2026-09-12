//go:build !linux

package textdocument

import (
	"context"
	"errors"
)

// ReadSnapshotFile is unavailable outside the selected Linux import boundary.
func ReadSnapshotFile(context.Context, string) ([]byte, error) {
	return nil, errors.New("text snapshot import requires the selected Linux platform")
}
