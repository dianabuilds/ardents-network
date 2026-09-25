//go:build linux

package qualification

import (
	"errors"
	"io"
	"sync"
)

// Attachment bridges an active-window resource sample and EOF. The verified
// worker still owns its original cgroup during sampling; the bridge then joins
// Service cleanup and the lifetime owner proves whole-process termination.
type Attachment struct {
	io.ReadWriteCloser
	sample func() error
	once   sync.Once
	err    error
}

func NewAttachment(conn io.ReadWriteCloser, sample func() error) *Attachment {
	return &Attachment{ReadWriteCloser: conn, sample: sample}
}

func (attachment *Attachment) Close() error {
	attachment.once.Do(func() {
		var sampleErr error
		if attachment.sample != nil {
			sampleErr = attachment.sample()
		}
		attachment.err = errors.Join(sampleErr, attachment.ReadWriteCloser.Close())
	})
	return attachment.err
}