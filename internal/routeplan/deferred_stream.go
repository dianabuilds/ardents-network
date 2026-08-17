package routeplan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// deferredUnixStream delays Endpoint registration until the publisher has an
// authenticated upstream Carrier. Step.Close remains its single owner.
type deferredUnixStream struct {
	path        string
	timeout     time.Duration
	mu          sync.Mutex
	stream      net.Conn
	attachments map[net.Conn]struct{}
	closed      bool
	writeClosed bool
}

func (value *deferredUnixStream) Read(buffer []byte) (int, error) {
	stream, err := value.open()
	if err != nil {
		return 0, err
	}
	return stream.Read(buffer)
}

func (value *deferredUnixStream) Write(buffer []byte) (int, error) {
	stream, err := value.open()
	if err != nil {
		return 0, err
	}
	return stream.Write(buffer)
}

func (value *deferredUnixStream) Close() error {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.closed {
		return net.ErrClosed
	}
	value.closed = true
	if value.stream == nil {
		return value.closeAttachments()
	}
	return errors.Join(value.stream.Close(), value.closeAttachments())
}

// OpenAttachment creates one separately owned publisher IPC stream for a
// bounded concurrent Route Attachment.
func (value *deferredUnixStream) OpenAttachment(ctx context.Context) (io.ReadWriteCloser, error) {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.closed || value.writeClosed {
		return nil, net.ErrClosed
	}
	if len(value.attachments) >= 16 {
		return nil, errors.New("publisher attachment stream capacity is full")
	}
	dialer := net.Dialer{Timeout: value.timeout}
	stream, err := dialer.DialContext(ctx, "unix", value.path)
	if err != nil {
		return nil, err
	}
	if value.attachments == nil {
		value.attachments = make(map[net.Conn]struct{}, 16)
	}
	value.attachments[stream] = struct{}{}
	return stream, nil
}

func (value *deferredUnixStream) closeAttachments() error {
	var err error
	for stream := range value.attachments {
		err = errors.Join(err, stream.Close())
		delete(value.attachments, stream)
	}
	return err
}

func (value *deferredUnixStream) CloseWrite() error {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.closed {
		return net.ErrClosed
	}
	value.writeClosed = true
	if value.stream == nil {
		return nil
	}
	return closeDeferredWrite(value.stream)
}

func (value *deferredUnixStream) open() (net.Conn, error) {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.closed {
		return nil, net.ErrClosed
	}
	if value.stream != nil {
		return value.stream, nil
	}
	stream, err := net.DialTimeout("unix", value.path, value.timeout)
	if err != nil {
		return nil, err
	}
	if value.writeClosed {
		if err := closeDeferredWrite(stream); err != nil {
			return nil, errors.Join(fmt.Errorf("apply deferred publisher stream half-close: %w", err), stream.Close())
		}
	}
	value.stream = stream
	return stream, nil
}

func closeDeferredWrite(stream net.Conn) error {
	closer, ok := stream.(interface{ CloseWrite() error })
	if !ok {
		return errors.New("publisher stream does not support half-close")
	}
	return closer.CloseWrite()
}
