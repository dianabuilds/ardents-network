//go:build linux || darwin

package publisher

import (
	"context"
	"io"
	"os"
)

type snapshotSource struct {
	file    *os.File
	size    int64
	staging *snapshotStaging
	release func()
}

// openReader borrows the original source while the Publication owns it.
// Each reader starts at offset zero without sharing the descriptor cursor.
func (source *snapshotSource) openReader() *io.SectionReader {
	return io.NewSectionReader(source.file, 0, source.size)
}

func (source *snapshotSource) close() error {
	if err := source.file.Close(); err != nil {
		return err
	}
	source.release()
	return nil
}

type snapshotStaging struct {
	cancel context.CancelFunc
	done   chan struct{}
}
