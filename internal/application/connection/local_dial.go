package connection

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// setupCancellation owns a local transport until Dial transfers that ownership
// to a Client. The handoff and cancellation compete under one lock, so an
// already-started cancellation cannot be lost between the accepted status and
// the returned Client.
type setupCancellation struct {
	mu       sync.Mutex
	canceled bool
	cancel   func()
}

func (guard *setupCancellation) close() {
	guard.mu.Lock()
	guard.canceled = true
	cancel := guard.cancel
	guard.mu.Unlock()
	cancel()
}

func (guard *setupCancellation) handoff(cancel func()) bool {
	guard.mu.Lock()
	defer guard.mu.Unlock()
	if guard.canceled {
		return false
	}
	guard.cancel = cancel
	return true
}

// Dial requests one Target Link and returns no Target, State, Entry, Route,
// credential, or administration handle.
func Dial(ctx context.Context, path string, destination Request) (Client, error) {
	if ctx == nil || path == "" || validRequest(destination) != nil {
		return nil, errors.New("local Application dial input is invalid")
	}
	request, err := EncodeRequest(destination)
	if err != nil {
		return nil, err
	}
	return dialLocalStream(ctx, path, request, destination.Application == ByteStream)
}

// DialPublishedStream requests the fixed byte-stream profile through local
// Service Administration. Its established exchange uses the same byte frames.
func DialPublishedStream(ctx context.Context, path string) (Client, error) {
	if ctx == nil || path == "" {
		return nil, errors.New("local Application dial input is invalid")
	}
	return dialLocalStream(ctx, path, []byte(PublishByteStreamRequest), true)
}

func dialLocalStream(ctx context.Context, path string, request []byte, directionalEOF bool) (Client, error) {
	raw, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	guard := &setupCancellation{cancel: func() { _ = raw.Close() }}
	stopCancellation := context.AfterFunc(ctx, guard.close)
	setupOwnsTransport := true
	defer func() {
		if setupOwnsTransport {
			stopCancellation()
			_ = raw.Close()
		}
	}()
	connection, ok := raw.(*net.UnixConn)
	if !ok {
		return nil, errors.New("local Application attachment is not a Unix connection")
	}
	if deadline, available := ctx.Deadline(); available {
		_ = connection.SetDeadline(deadline)
	}
	if _, err := connection.Write(request); err != nil {
		return nil, setupError(ctx, err)
	}
	var status [1]byte
	if _, err := io.ReadFull(connection, status[:]); err != nil {
		return nil, setupError(ctx, errors.New("local Application Connection is unavailable"))
	}
	if status[0] != 1 {
		outcome, refusalErr := readRefusal(connection)
		if refusalErr != nil {
			return nil, setupError(ctx, errors.New("local Application Connection is unavailable"))
		}
		return nil, setupError(ctx, SetupRefusalError{outcome: outcome})
	}
	_ = connection.SetDeadline(time.Time{})
	opened := newClientWithStop(connection, stopCancellation)
	opened.directionalEOF = directionalEOF
	go opened.receive()
	if !guard.handoff(func() { _ = opened.Close() }) {
		_ = opened.Close()
		return nil, setupError(ctx, errors.New("local Application Connection is unavailable"))
	}
	if err := ctx.Err(); err != nil {
		_ = opened.Close()
		return nil, err
	}
	setupOwnsTransport = false
	return opened, nil
}

func setupError(ctx context.Context, fallback error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, available := ctx.Deadline(); available && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return fallback
}
