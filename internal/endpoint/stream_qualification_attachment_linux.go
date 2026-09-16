//go:build linux

package endpoint

import (
	"errors"
	"io"
	"sync"
)

// The final active-window sample precedes attachment EOF, while the verified
// worker still owns its original cgroup. The bridge then joins Service cleanup
// and the lifetime owner proves whole-process termination.
type qualificationAttachment struct {
	io.ReadWriteCloser
	sample func() error
	once   sync.Once
	err    error
}

func (attachment *qualificationAttachment) Close() error {
	attachment.once.Do(func() {
		var sampleErr error
		if attachment.sample != nil {
			sampleErr = attachment.sample()
		}
		attachment.err = errors.Join(sampleErr, attachment.ReadWriteCloser.Close())
	})
	return attachment.err
}
